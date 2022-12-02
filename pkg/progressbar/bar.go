package progressbar

import (
	"fmt"
	"github.com/timerzz/g3u8d/pkg/utils"
	"time"
)

type cfg struct {
	interval   time.Duration
	stepHook   func(*Bar)
	finishHook func()
	title      string
}

type Bar struct {
	total    int64
	cur      int64
	size     int64
	lastSize int64
	lastTime time.Time

	cfg cfg

	finish chan struct{}
}

func New(opts ...Option) *Bar {
	var c cfg
	for _, opt := range opts {
		opt(&c)
	}
	return &Bar{
		cfg:    c,
		finish: make(chan struct{}),
	}
}
func (b *Bar) Run() {
	ticker := time.NewTicker(b.cfg.interval)
	defer ticker.Stop()
	b.lastTime = time.Now()
	for {
		select {
		case <-ticker.C:
			if b.cfg.stepHook != nil {
				b.cfg.stepHook(b)
			}
			if b.total <= 0 {
				continue
			}
			b.render()
			b.lastTime = time.Now()
		case <-b.finish:
			if b.cfg.stepHook != nil {
				b.cfg.stepHook(b)
			}
			b.render()
			if b.cfg.finishHook != nil {
				b.cfg.finishHook()
			}
			return
		}
	}
}

func (b *Bar) render() {
	fmt.Printf("\r %s %.2f%%  %8d/%d %30s", b.cfg.title, 100*float64(b.cur)/float64(b.total), b.cur, b.total, utils.SizePercentFormat(b.lastTime, b.size-b.lastSize))
}

func (b *Bar) SetTotal(t int64) {
	b.total = t
}

func (b *Bar) SetCur(t int64) {
	b.cur = t
}

func (b *Bar) SetSize(t int64) {
	b.lastSize, b.size = b.size, t
}

func (b *Bar) Finish() {
	b.finish <- struct{}{}
}

type Option func(*cfg)

func WithInterval(duration time.Duration) func(*cfg) {
	return func(cfg *cfg) {
		cfg.interval = duration
	}
}

func WithTitle(title string) func(*cfg) {
	return func(cfg *cfg) {
		cfg.title = title
	}
}

func WithStepHook(h func(self *Bar)) func(*cfg) {
	return func(cfg *cfg) {
		cfg.stepHook = h
	}
}

func WithFinishHook(h func()) func(*cfg) {
	return func(cfg *cfg) {
		cfg.finishHook = h
	}
}
