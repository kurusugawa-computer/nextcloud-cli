package download

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	_path "path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/thamaji/pbpool"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/cheggaaa/pb.v1"
)

type Option func(*ctx) error

const (
	DeconflictError     = "error"
	DeconflictSkip      = "skip"
	DeconflictOverwrite = "overwrite"
	DeconflictNewest    = "newest"
)

func DeconflictStrategy(strategy string) Option {
	return func(ctx *ctx) error {
		switch strategy {
		case DeconflictError:
			ctx.deconflictStrategy = 0

		case DeconflictSkip:
			ctx.deconflictStrategy = 1

		case DeconflictOverwrite:
			ctx.deconflictStrategy = 2

		case DeconflictNewest:
			ctx.deconflictStrategy = 3

		default:
			return errors.New("invalid strategy: " + strategy)
		}

		return nil
	}
}

func Retry(n int, delay time.Duration) Option {
	return func(ctx *ctx) error {
		if n < 0 {
			return fmt.Errorf("invalid retry count: %d", n)
		}

		if delay < 0 {
			return fmt.Errorf("invalid delay: %s", delay)
		}

		ctx.retry = n
		ctx.delay = delay

		return nil
	}
}

func Procs(n int) Option {
	return func(ctx *ctx) error {
		if n <= 0 {
			return errors.New("procs should 1<=")
		}

		if ctx.sem != nil {
			close(ctx.sem)
		}

		ctx.sem = make(chan struct{}, n)

		return nil
	}
}

func Do(n *nextcloud.Nextcloud, opts []Option, srcs []string, dst string) error {
	ctx := &ctx{
		n: n,

		sem: make(chan struct{}, 2),
		wg:  &sync.WaitGroup{},

		done: 0,
		m:    &sync.Mutex{},
		err:  nil,

		pool: nil,

		deconflictStrategy: 0,

		retry: 3,
		delay: 30 * time.Second,
	}

	for _, opt := range opts {
		if err := opt(ctx); err != nil {
			return err
		}
	}

	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		ctx.pool = pbpool.New()
	}

	if ctx.pool != nil {
		ctx.pool.Start()
	}

	for _, src := range srcs {
		fi, err := ctx.n.Stat(src)
		if err != nil {
			return err
		}

		download(ctx, src, fi, dst)
	}

	ctx.wg.Wait()

	if ctx.pool != nil {
		ctx.pool.Update()
		ctx.pool.Stop()
	}

	return ctx.err
}

type ctx struct {
	n *nextcloud.Nextcloud // Nextcloud クライアント

	sem chan struct{}   // 並列数を制御するためのセマフォとして扱う chan
	wg  *sync.WaitGroup // すべてのダウンロードが終わるまで待つための WaitGroup

	done uint32      // エラーなどで中断していたら done == 1。atomic 経由で読み書きすべし
	m    *sync.Mutex // err を更新するときのミューテックス
	err  error       // 処理中に起きた最初のエラー

	pool *pbpool.Pool // プログレスバーのプール

	deconflictStrategy int // ファイルが衝突したときの処理方法

	retry int           // リトライ回数
	delay time.Duration // リトライ時のディレイ
}

func (ctx *ctx) setError(err error) {
	if atomic.LoadUint32(&(ctx.done)) == 1 {
		return
	}

	ctx.m.Lock()
	if ctx.err == nil {
		ctx.err = err
	}
	atomic.StoreUint32(&(ctx.done), 1)
	ctx.m.Unlock()
}

func download(ctx *ctx, src string, fi os.FileInfo, dst string) {
	if atomic.LoadUint32(&(ctx.done)) == 1 {
		return // エラーなどで中断(ctx.done == 1)していたらあたらしい処理を行わない
	}

	if fi.IsDir() {
		dst = filepath.Join(dst, fi.Name())

		if err := os.MkdirAll(dst, fi.Mode()); err != nil {
			ctx.setError(err)
			return
		}

		fl, err := ctx.n.ReadDir(src)
		if err != nil {
			ctx.setError(err)
			return
		}

		for _, fi := range fl {
			download(ctx, _path.Join(src, fi.Name()), fi, dst)
		}

		return
	}

	dir := dst
	dst = filepath.Join(dst, fi.Name())

	ctx.sem <- struct{}{}
	ctx.wg.Add(1)
	go func() {
		defer func() {
			ctx.wg.Done()
			<-ctx.sem
		}()

		switch ctx.deconflictStrategy {
		case 0: // DeconflictError
			if _, err := os.Stat(dst); err == nil {
				ctx.setError(errors.New("local file already exists: " + dst))
				return
			}

		case 1: // DeconflictSkip
			if _, err := os.Stat(dst); err == nil {
				fmt.Println("skip already exists file: " + src)
				return
			}

		case 2: // DeconflictOverwrite

		case 3: // DeconflictNewest
			if fi1, err := os.Stat(dst); err == nil && !fi.ModTime().After(fi1.ModTime()) {
				fmt.Println("skip older file: " + src)
				return
			}
		}

		n := 0
		for {
			err := downloadFile(ctx, dir, src, fi, dst)
			if err == nil {
				return
			}

			n++
			if ctx.retry > 0 && ctx.retry > n {
				fmt.Println("error! retry after " + ctx.delay.String() + "...")
				fmt.Println("  " + err.Error())
				time.Sleep(ctx.delay)
				continue
			}

			ctx.setError(err)
			return
		}
	}()
}

func downloadFile(ctx *ctx, dir, src string, fi os.FileInfo, dst string) error {
	var bar *pbpool.ProgressBar

	if ctx.pool == nil {
		fmt.Fprintln(os.Stdout, src)

	} else {
		bar = ctx.pool.Get()
		bar.SetTotal64(fi.Size())
		bar.Prefix(src)
		bar.SetUnits(pb.U_BYTES)
		bar.Start()
		defer func() {
			bar.Finish()
			ctx.pool.Put(bar)
		}()
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		if err1 := os.MkdirAll(dir, 0775); err1 != nil {
			return err
		}

		dstFile, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
		if err != nil {
			return err
		}
	}

	srcFile, err := ctx.n.ReadFile(src)
	if err != nil {
		dstFile.Close()
		return err
	}

	var w io.Writer = dstFile
	if bar != nil {
		w = io.MultiWriter(dstFile, bar)
	}

	// 並列で書き込みするときにディスクのIO待ちを軽減しようと思って buffered writer にした
	// 計測してないので、必要ないかもしれない
	bw := bufio.NewWriter(w)

	_, err = io.Copy(bw, srcFile)

	if err1 := bw.Flush(); err1 != nil {
		err = err1
	}

	if err1 := dstFile.Sync(); err1 != nil {
		err = err1
	}

	srcFile.Close()
	dstFile.Close()

	return err
}
