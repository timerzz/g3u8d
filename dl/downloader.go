package dl

import (
	"bytes"
	"fmt"
	"github.com/grafov/m3u8"
	"github.com/imroc/req/v3"
	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
	"github.com/timerzz/g3u8d/pkg/decode"
	"github.com/timerzz/nio"
	"golang.org/x/net/context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type DownLoader struct {
	key []byte
	tmp string

	cfg    Config //配置
	chunks []*chunk

	m3u8key  *m3u8.Key
	pool     *ants.Pool         //协程池
	client   *req.Client        //http客户端
	playList m3u8.MediaPlaylist // 媒体列表

	cancel context.CancelFunc
	ctx    context.Context

	downloadSize int64 //每秒钟下载的数量

	total    int64 //总共需要下载的数量
	complete int64 //完成的下载数量

	retryChannel chan *chunk

	saveFile *os.File
}

func New(cfg Config) *DownLoader {
	client := req.C()
	if cfg.Proxy != "" {
		client = client.SetProxyURL(cfg.Proxy)
	}
	if cfg.RetryCount > 0 {
		client = client.SetCommonRetryCount(cfg.RetryCount)
	}
	if cfg.Timeout != nil {
		client = client.SetTimeout(*cfg.Timeout).SetTLSHandshakeTimeout(*cfg.Timeout)
	}
	if cfg.BaseUrl == "" && cfg.M3u8Url != "" {
		cfg.BaseUrl = strings.TrimRight(cfg.M3u8Url, path.Base(cfg.M3u8Url))
	}
	if cfg.BaseUrl != "" {
		client = client.SetBaseURL(cfg.BaseUrl)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &DownLoader{
		cfg:    cfg,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (d *DownLoader) Wait() <-chan struct{} {
	ctx, _ := context.WithCancel(d.ctx)
	return ctx.Done()
}

// 下载并解析m3u8文件
func (d *DownLoader) parse() error {
	var rc io.ReadCloser
	defer func() {
		_ = rc.Close()
	}()
	if d.cfg.M3u8Url != "" {
		resp, err := d.client.R().Get(d.cfg.M3u8Url)
		if err != nil {
			return fmt.Errorf("下载m3u8文件失败：%v", err)
		}
		rc = resp.Body
	} else if d.cfg.M3u8Path != "" {
		file, err := os.Open(d.cfg.M3u8Path)
		if err != nil {
			return fmt.Errorf("打开%s失败：%v", d.cfg.M3u8Path, err)
		}
		rc = file
	}

	playlist, listType, err := m3u8.DecodeFrom(rc, true)
	if err != nil {
		return fmt.Errorf("m3u8解析失败：%v", err)
	}
	switch listType {
	case m3u8.MEDIA:
		d.playList = *(playlist.(*m3u8.MediaPlaylist))
	case m3u8.MASTER:
		return fmt.Errorf("暂不支持该类型m3u8文件")
	}
	if d.playList.Key != nil {
		d.m3u8key = d.playList.Key
	}
	return nil
}

func (d *DownLoader) requestKey() (err error) {
	if d.m3u8key != nil {
		var w = bytes.NewBuffer(make([]byte, 0, 32))
		if _, err = d.client.R().SetOutput(w).Get(d.m3u8key.URI); err != nil {
			err = fmt.Errorf("请求key失败：%v", err)
		}
		d.key = w.Bytes()
	}
	return err
}

func (d *DownLoader) Run() (err error) {
	if err = d.parse(); err != nil {
		return
	}

	if err = d.mkTmp(); err != nil {
		return
	}

	if err = d.requestKey(); err != nil {
		return
	}

	if d.pool, err = ants.NewPool(d.cfg.MaxThread); err != nil {
		return
	}

	d.initChunk()

	// 初始化channel
	d.retryChannel = make(chan *chunk, d.cfg.MaxThread)
	defer close(d.retryChannel)

	go d.retryRunner()

	go func() {
		for _, chunk := range d.chunks {
			var c = chunk
			if d.ctx.Err() == nil {
				if err := d.pool.Submit(func() {
					if err = d.downloadChunk(c); err != nil {
						logrus.Errorf("下载失败！%v", err)
						d.cancel()
					}
				}); err != nil {
					logrus.Error(err)
				}
			}
		}
	}()
	return d.merge()
}

func (d *DownLoader) mkTmp() (err error) {
	d.tmp = filepath.Join(d.cfg.WorkDr, fmt.Sprintf("%s.tmp", d.cfg.SaveName))
	if err = os.MkdirAll(d.tmp, 0644); err != nil {
		return fmt.Errorf("创建临时目录失败:%v", err)
	}
	return
}

// 进行合并
func (d *DownLoader) merge() (err error) {
	defer d.cancel()
	// 保存的文件
	if d.saveFile, err = os.Create(filepath.Join(d.cfg.WorkDr, d.cfg.SaveName)); err != nil {
		return
	}

	defer func() {
		_ = d.saveFile.Close()
	}()

	for _, chunk := range d.chunks {
		select {
		case <-d.ctx.Done():
			return nil
		case <-chunk.ctx.Done():
		}

		err := func() error {
			f, err := os.Open(chunk.filepath)
			if err != nil {
				return err
			}
			_, err = io.Copy(d.saveFile, f)
			_ = f.Close()
			_ = os.Remove(chunk.filepath)
			return err

		}()
		if err != nil {
			return err
		}
	}
	for os.RemoveAll(d.tmp) != nil {

	}
	return nil
}

func (d *DownLoader) initChunk() {
	d.chunks = make([]*chunk, 0, d.playList.Count())
	idx := 0
	for _, seg := range d.playList.Segments {
		if seg != nil {
			chunk := &chunk{
				index: idx,
				url:   seg.URI,
			}
			chunk.ctx, chunk.cancel = context.WithCancel(d.ctx)
			d.chunks = append(d.chunks, chunk)
			idx++
		}
	}
	d.total = int64(d.playList.Count())
}

func (d *DownLoader) downloadChunk(chunk *chunk) error {
	// 已经下载完了，不处理
	if chunk.status == chunk_status_merged || chunk.status == chunk_status_success {
		return nil
	}

	// 以前下载过了,直接标记为成功
	finialFile := filepath.Join(d.tmp, fmt.Sprintf("%d.ts", chunk.index))
	if _, _err := os.Stat(finialFile); _err == nil {
		atomic.AddInt64(&d.complete, 1)
		chunk.lock.Lock()
		chunk.status = chunk_status_success
		chunk.cancel()
		chunk.filepath = finialFile
		chunk.lock.Unlock()
		return nil
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp(d.tmp, fmt.Sprintf("%d_*.ts", chunk.index))
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpFile.Name())
	}()

	if chunk.status == chunk_status_merged || chunk.status == chunk_status_success {
		return tmpFile.Close()
	}

	go func() {
		// 如果超时时间过了还没完成，那就重试
		timer := time.NewTimer(*d.cfg.Timeout)
		select {
		case <-timer.C:
			atomic.AddInt32(&chunk.retryTimes, 1)
			d.retryChannel <- chunk
		case <-chunk.ctx.Done():
		}
	}()

	_, err = d.client.R().SetOutput(nio.NWriter(tmpFile, func(n int) { d.downloadSize += int64(n) })).Get(chunk.url)

	_ = tmpFile.Close()

	if err != nil {
		if chunk.retryTimes < int32(d.cfg.RetryCount) {
			atomic.AddInt32(&chunk.retryTimes, 1)
			d.retryChannel <- chunk
		} else {
			return err
		}
	}

	// 下载成功了
	chunk.lock.Lock()
	defer chunk.lock.Unlock()

	if chunk.status == chunk_status_merged || chunk.status == chunk_status_success {
		return nil
	}

	chunk.status = chunk_status_success
	if d.m3u8key != nil {
		var b []byte
		if b, err = os.ReadFile(tmpFile.Name()); err != nil {
			return fmt.Errorf("读取 %s 失败：%v", tmpFile.Name(), err)
		}

		b, err = decode.AESDecrypt(b, d.key, []byte(d.m3u8key.IV))
		if err != nil {
			return err
		}
		err = func() error {
			f, err := os.Create(finialFile)
			if err != nil {
				return err
			}
			defer func() {
				_ = f.Close()
			}()
			_, err = f.Write(b)
			return err
		}()
		if err != nil {
			return fmt.Errorf("保存 %s 失败： %v", finialFile, err)
		}
	} else {
		if err = os.Rename(tmpFile.Name(), finialFile); err != nil {
			return fmt.Errorf("重命名%s失败：%v", tmpFile.Name(), err)
		}
	}

	chunk.filepath = finialFile
	atomic.AddInt64(&d.complete, 1)
	chunk.cancel()

	return nil
}

// DownloadSize 已经下载的大小
func (d *DownLoader) DownloadSize() int64 {
	return d.downloadSize
}

// Progress 获取当前下载数量
func (d *DownLoader) Progress() (int64, int64) {
	return d.complete, d.total
}

func (d *DownLoader) retryRunner() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case chunk := <-d.retryChannel:
			if chunk == nil {
				if d.ctx.Err() != nil {
					return
				}
				continue
			}
			if err := d.pool.Submit(func() {
				if err := d.downloadChunk(chunk); err != nil {
					logrus.Errorf("下载失败！%v", err)
					d.cancel()
				}
			}); err != nil {
				logrus.Error(err)
			}
		}
	}
}
