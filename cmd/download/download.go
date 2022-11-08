package download

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	_path "path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/thamaji/pbpool"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/cheggaaa/pb.v1"
)

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

	join bool // 分割されていそうなファイルが存在したときに自動で結合するかどうか
}

type Option func(*ctx) error

const (
	DeconflictError     = "error"
	DeconflictSkip      = "skip"
	DeconflictOverwrite = "overwrite"
	DeconflictNewest    = "newest"
	DeconflictLarger    = "larger"
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

		case DeconflictLarger:
			ctx.deconflictStrategy = 4

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

func Join(b bool) Option {
	return func(ctx *ctx) error {
		ctx.join = b
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

		join: false,
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
		download(ctx, src, dst)
	}

	ctx.wg.Wait()

	if ctx.pool != nil {
		ctx.pool.Update()
		ctx.pool.Stop()
	}

	return ctx.err
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

func download(ctx *ctx, src string, dst string) {
	if atomic.LoadUint32(&(ctx.done)) == 1 {
		return // エラーなどで中断(ctx.done == 1)していたらあたらしい処理を行わない
	}

	if src == "/" {
		_downloadDir(ctx, src, dst)
		return
	}

	if ctx.join {
		// joinした後に同じsrcという名前になるものがないかチェックする
		fisMap, err := ctx.n.ReadJoinedDir(_path.Dir(src))
		if err != nil {
			ctx.setError(err)
			return
		}

		fls := fisMap[_path.Base(src)]
		if len(fls) == 0 {
			fi, err := ctx.n.Stat(src)
			if err != nil {
				ctx.setError(err)
				return
			}
			ctx.setError(fmt.Errorf("unexpected: %s not found in %s by ReadJoinedDir, but actually exists", fi.Name(), _path.Dir(src)))
			return
		}

		if len(fls) != 1 {
			// joinした後に同じsrcという名前になるものが複数存在する
			names := []string{}
			for _, fis := range fls {
				for _, fi := range fis {
					names = append(names, fi.Name())
				}
			}
			ctx.setError(fmt.Errorf("name collision detected: %s", strings.Join(names, " ")))
			return
		}

		// joinした後にsrcとなるものがただ一つ存在する。
		if fls[0][0].IsDir() {
			_downloadDir(ctx, src, dst)
			return
		}

		srcs := []string{}
		for _, fi := range fls[0] {
			srcs = append(srcs, _path.Join(_path.Dir(src), fi.Name()))
		}
		if err := _downloadAndJoinFiles(ctx, dst, srcs, filepath.Join(dst, filepath.Base(src))); err != nil {
			ctx.setError(err)
			return
		}

		return
	}

	fi, err := ctx.n.Stat(src)
	if err != nil {
		ctx.setError(err)
		return
	}

	if fi.IsDir() {
		if err := os.MkdirAll(filepath.Join(dst, src), fi.Mode()); err != nil {
			ctx.setError(err)
			return
		}
		_downloadDir(ctx, src, filepath.Join(dst, src))
		return
	}
	if err := _downloadFile(ctx, dst, src, filepath.Join(dst, fi.Name())); err != nil {
		ctx.setError(err)
		return
	}
}

func _downloadDir(ctx *ctx, src string, dst string) {
	fi, err := ctx.n.Stat(src)
	if err != nil {
		ctx.setError(err)
		return
	}

	if !fi.IsDir() {
		err := fmt.Errorf("unexpected: %s is expected to directory", src)
		ctx.setError(err)
		return
	}

	if err := os.MkdirAll(dst, fi.Mode()); err != nil {
		ctx.setError(err)
		return
	}

	type task struct {
		srcs []string
		dst  string
	}
	tasks := []*task{}

	// tasksの形に変形する
	if ctx.join {
		fisMap, err := ctx.n.ReadJoinedDir(src)
		if err != nil {
			ctx.setError(err)
			return
		}

		for name, fls := range fisMap {
			if len(fls) != 1 {
				// joinした後に同じsrcという名前になるものが複数存在する
				names := []string{}
				for _, fis := range fls {
					for _, fi := range fis {
						names = append(names, fi.Name())
					}
				}
				ctx.setError(fmt.Errorf("name collision detected: %s", strings.Join(names, " ")))
				return
			}

			fl := fls[0]

			if fl[0].IsDir() {
				_downloadDir(ctx, _path.Join(src, fl[0].Name()), filepath.Join(dst, fl[0].Name()))
				continue
			}

			task := &task{
				srcs: []string{},
				dst:  filepath.Join(dst, name),
			}
			for _, fi := range fl {
				task.srcs = append(task.srcs, _path.Join(src, fi.Name()))
			}
			tasks = append(tasks, task)
		}

	} else {
		fl, err := ctx.n.ReadDir(src)
		if err != nil {
			ctx.setError(err)
			return
		}

		for _, fi := range fl {
			if fi.IsDir() {
				_downloadDir(ctx, _path.Join(src, fi.Name()), filepath.Join(dst, fi.Name()))
				continue
			}
			tasks = append(tasks, &task{
				srcs: []string{_path.Join(src, fi.Name())},
				dst:  filepath.Join(dst, fi.Name()),
			})
		}
	}

	for _, task := range tasks {
		ctx.sem <- struct{}{}
		ctx.wg.Add(1)
		task := task
		go func() {
			defer func() {
				ctx.wg.Done()
				<-ctx.sem
			}()
			if err := _downloadAndJoinFiles(ctx, dst, task.srcs, task.dst); err != nil {
				ctx.setError(err)
				return
			}
		}()
	}
}

func _downloadFile(ctx *ctx, dir, src string, dst string) error {
	return _downloadAndJoinFiles(ctx, dir, []string{src}, dst)
}

// srcsのファイルを順番にdstに書き込む
func _downloadAndJoinFiles(ctx *ctx, dir string, srcs []string, dst string) error {
	if len(srcs) == 0 {
		return errors.New("unexpected: tried to download empty file set")
	}
	srcFirstFileInfo, err := ctx.n.Stat(srcs[0])
	if err != nil {
		return err
	}
	totalSize := int64(0)
	for _, src := range srcs {
		fi, err := ctx.n.Stat(src)
		if err != nil {
			return err
		}
		totalSize += fi.Size()
	}

	joinedFilename := _path.Join(_path.Dir(srcs[0]), _path.Base(dst))

	switch ctx.deconflictStrategy {
	case 0: // DeconflictError
		if _, err := os.Stat(dst); err == nil {
			return errors.New("local file already exists: " + dst)
		}

	case 1: // DeconflictSkip
		if _, err := os.Stat(dst); err == nil {
			fmt.Println("skip already exists file: " + joinedFilename)
			return nil
		}

	case 2: // DeconflictOverwrite

	case 3: // DeconflictNewest
		if fi1, err := os.Stat(dst); err == nil && !srcFirstFileInfo.ModTime().After(fi1.ModTime()) {
			fmt.Println("skip older file: " + joinedFilename)
			return nil
		}

	case 4: // DeconflictLarger
		if fi1, err := ctx.n.Stat(dst); err == nil && totalSize <= fi1.Size() {
			fmt.Println("skip not larger file: " + joinedFilename)
			return nil
		}
	}

	try := func() error {
		var bar *pbpool.ProgressBar

		if ctx.pool == nil {
			fmt.Fprintln(os.Stdout, joinedFilename)
		} else {
			bar = ctx.pool.Get()
			bar.SetTotal64(totalSize)
			bar.Prefix(joinedFilename)
			bar.SetUnits(pb.U_BYTES)
			bar.Start()
			defer func() {
				bar.Finish()
				ctx.pool.Put(bar)
			}()
		}

		dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcFirstFileInfo.Mode())
		if err != nil {
			if err1 := os.MkdirAll(dir, 0775); err1 != nil {
				return err
			}

			dstFile, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcFirstFileInfo.Mode())
			if err != nil {
				return err
			}
		}

		var w io.Writer = dstFile
		if bar != nil {
			w = io.MultiWriter(dstFile, bar)
		}

		// 並列で書き込みするときにディスクのIO待ちを軽減しようと思って buffered writer にした
		// 計測してないので、必要ないかもしれない
		bw := bufio.NewWriter(w)

		for _, src := range srcs {
			srcFile, err := ctx.n.ReadFile(src)
			if err != nil {
				dstFile.Close()
				return err
			}

			_, err = io.Copy(bw, srcFile)

			if err1 := bw.Flush(); err1 != nil {
				err = err1
			}

			if err1 := dstFile.Sync(); err1 != nil {
				err = err1
			}

			srcFile.Close()
			if err != nil {
				dstFile.Close()
				return err
			}
		}

		dstFile.Close()

		return err
	}

	n := 0
	for {
		err := try()
		if err == nil {
			return nil
		}

		n++
		if ctx.retry > 0 && ctx.retry > n {
			fmt.Println("error! retry after " + ctx.delay.String() + "...")
			fmt.Println("  " + err.Error())
			time.Sleep(ctx.delay)
			continue
		}

		return err
	}
}
