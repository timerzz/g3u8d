package main

import (
	"fmt"
	"github.com/timerzz/g3u8d/dl"
	"github.com/timerzz/g3u8d/pkg/progressbar"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"time"
)

func main() {
	defaultTimeout := time.Second * time.Duration(15)
	var c = dl.Config{
		RetryCount: 5,
		MaxThread:  64,
		SaveName:   "file.mp4",
		Timeout:    &defaultTimeout,
	}
	c.WorkDr, _ = os.Getwd()
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:        "r",
				Aliases:     []string{"retry"},
				Value:       5,
				Usage:       "设置重试次数",
				Destination: &c.RetryCount,
			},
			&cli.IntFlag{
				Name:    "t",
				Value:   15,
				Aliases: []string{"timeout"},
				Usage:   "设置超时时间",
				Action: func(context *cli.Context, i int) error {
					timeout := time.Second * time.Duration(i)
					c.Timeout = &timeout
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "u",
				Usage:       "设置要下载的m3u8的url",
				Destination: &c.M3u8Url,
			},
			&cli.StringFlag{
				Name:        "p",
				Usage:       "设置要下载的m3u8的文件",
				Destination: &c.M3u8Path,
			},
			&cli.StringFlag{
				Name:        "f",
				Usage:       "设置保存的文件名称",
				Value:       "file.mp4",
				Destination: &c.SaveName,
			},
			&cli.StringFlag{
				Name:        "d",
				Aliases:     []string{"dir"},
				Usage:       "设置保存的目录",
				Destination: &c.WorkDr,
			},
			&cli.StringFlag{
				Name:        "b",
				Aliases:     []string{"base"},
				Usage:       "设置base url",
				Destination: &c.BaseUrl,
			},
			&cli.StringFlag{
				Name:        "proxy",
				Usage:       "设置使用的代理，格式如：http://localhost:3000",
				Destination: &c.Proxy,
			},
			&cli.IntFlag{
				Name:        "thread",
				Value:       64,
				Usage:       "设置最大并发数",
				Destination: &c.MaxThread,
			},
		},
		Action: func(context *cli.Context) error {
			d := dl.New(c)
			go d.Run()

			bar := progressbar.New(
				progressbar.WithInterval(time.Second),
				progressbar.WithStepHook(func(b *progressbar.Bar) {
					cur, total := d.Progress()
					b.SetSize(d.DownloadSize())
					b.SetCur(cur)
					b.SetTotal(total)
				}),
				progressbar.WithTitle("正在下载"),
				progressbar.WithFinishHook(func() {
					fmt.Printf("\n 结束下载...")
				}),
			)
			go bar.Run()
			wait := d.Wait()
			<-wait
			bar.Finish()
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
