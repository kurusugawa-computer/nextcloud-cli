package upload

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	_path "path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/cheggaaa/pb.v1"

	"github.com/c2h5oh/datasize"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/thamaji/pbpool"
	"golang.org/x/crypto/ssh/terminal"
)

type Option func(*ctx) error

const (
	DeconflictError     = 0
	DeconflictSkip      = 1
	DeconflictOverwrite = 2
	DeconflictNewest    = 3
	DeconflictLarger    = 4
)

func DeconflictStrategy(strategy string) Option {
	return func(ctx *ctx) error {
		switch strategy {
		case "error":
			ctx.deconflictStrategy = DeconflictError

		case "skip":
			ctx.deconflictStrategy = DeconflictSkip

		case "overwrite":
			ctx.deconflictStrategy = DeconflictOverwrite

		case "newest":
			ctx.deconflictStrategy = DeconflictNewest

		case "larger":
			ctx.deconflictStrategy = DeconflictLarger

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

func SplitSize(threshold string) Option {
	return func(ctx *ctx) error {
		var bytesize datasize.ByteSize
		err := bytesize.UnmarshalText([]byte(threshold))
		if err != nil {
			return errors.Wrapf(err, "failed to unmarshal %#v", threshold)
		}
		ctx.splitSize = int64(bytesize.Bytes())
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

		deconflictStrategy: DeconflictError,

		retry: 3,
		delay: 30 * time.Second,
	}

	for i, opt := range opts {
		if err := opt(ctx); err != nil {
			return errors.Wrapf(err, "failed to parse option %d", i)
		}
	}

	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		ctx.pool = pbpool.New()
	}

	if ctx.pool != nil {
		ctx.pool.Start()
	}

	var files []file

	for _, src := range srcs {
		fi, err := os.Stat(src)
		if err != nil {
			return errors.Wrapf(err, "error occurred while statting %#v", src)
		}

		if filedir, err := upload(ctx, src, fi, dst); err != nil {
			return errors.Wrapf(err, "error occurred while statting %#v", src)
		} else {
			files = append(files, filedir...)
		}
	}

	uploadFragment(files)

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

	splitSize int64 // このバイト数を超えないようにファイルを分割する。0なら無視
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

// getFileInfo 分割されたファイルも探すStat。
func getFileInfo(ctx *ctx, path string) ([]string, []os.FileInfo, error) {
	if fi, err := ctx.n.Stat(path); err == nil {
		return []string{path}, []os.FileInfo{fi}, nil
	}
	firstSplittedFile := path + ".000"
	fi, err := ctx.n.Stat(firstSplittedFile)
	if err != nil {
		return nil, nil, errors.Wrapf(err,
			"error occurred while statting of splitted file %#v", firstSplittedFile,
		)
	}
	result := []os.FileInfo{fi}
	paths := []string{firstSplittedFile}
	i := 1
	for {
		splittedPath := fmt.Sprintf("%s.%03d", path, i)
		fi, err = ctx.n.Stat(splittedPath)
		if err != nil {
			return paths, result, nil
		}
		result = append(result, fi)
		paths = append(paths, splittedPath)
		i++
	}
}

// getFullSize ファイルのバイト数を得る。分割されたファイルの場合は合計のサイズを計算する。
func getFullSize(ctx *ctx, fis []os.FileInfo) int64 {
	var sum int64 = 0
	for _, fi := range fis {
		sum += fi.Size()
	}
	return sum
}

func upload(ctx *ctx, src string, fi os.FileInfo, dst string) ([]file, error) {
	/*
		if atomic.LoadUint32(&(ctx.done)) == 1 {
			return nil, nil // エラーなどで中断(ctx.done == 1)していたらあたらしい処理を行わない
		}
	*/

	var files []file

	if fi.IsDir() {
		dst = _path.Join(dst, fi.Name())

		if err := ctx.n.MkdirAll(dst); err != nil {
			ctx.setError(
				errors.Wrapf(err,
					"recursive mkdir for destination directory %#v failed", dst,
				),
			)
			return nil, err
		}

		fl, err := ioutil.ReadDir(src)
		if err != nil {
			ctx.setError(
				errors.Wrapf(err,
					"failed to read source directory %#v", src,
				),
			)
			return nil, err
		}

		for _, fi := range fl {
			if file, err := upload(ctx, filepath.Join(src, fi.Name()), fi, dst); err != nil {
				return nil, err
			} else {
				files = append(files, file...)
			}
		}

		return files, nil
	}

	dir := dst
	dst = _path.Join(dst, fi.Name())

	switch ctx.deconflictStrategy {
	case DeconflictError:
		if _, _, err := getFileInfo(ctx, dst); err == nil {
			ctx.setError(errors.New("remote file already exists: " + dst))
			return nil, err
		} else if !os.IsNotExist(errors.Cause(err)) {
			ctx.setError(
				errors.Wrap(err, "getFileInfo in handling deconflict failed"),
			)
			return nil, err
		}

	case DeconflictSkip:
		if _, _, err := getFileInfo(ctx, dst); err == nil {
			fmt.Println("skip already exists file: " + src)
			return nil, err
		} else if !os.IsNotExist(errors.Cause(err)) {
			ctx.setError(
				errors.Wrap(err, "getFileInfo in handling deconflict failed"),
			)
			return nil, err
		}

	case DeconflictOverwrite:

	case DeconflictNewest:
		if _, fis, err := getFileInfo(ctx, dst); err == nil && !fi.ModTime().After(fis[0].ModTime()) {
			fmt.Println("skip older file: " + src)
			return nil, err
		} else if !os.IsNotExist(errors.Cause(err)) {
			ctx.setError(
				errors.Wrap(err, "getFileInfo in handling deconflict failed"),
			)
			return nil, err
		}

	case DeconflictLarger:
		if _, fis, err := getFileInfo(ctx, dst); err == nil && fi.Size() <= getFullSize(ctx, fis) {
			fmt.Println("skip not larger file: " + src)
			return nil, err
		} else if !os.IsNotExist(errors.Cause(err)) {
			ctx.setError(
				errors.Wrap(err, "getFileInfo in handling deconflict failed"),
			)
			return nil, err
		}
	}

	var size int64
	var offset int64

	if 0 < ctx.splitSize && ctx.splitSize < fi.Size() {
		for i := int64(0); i*ctx.splitSize < fi.Size(); i++ {
			offset = i * ctx.splitSize
			if fi.Size() < (i+1)*ctx.splitSize {
				size = int64(fi.Size())
			} else {
				size = (i + 1) * ctx.splitSize
			}
			size -= i * ctx.splitSize
			srcFile, err := open(
				ctx,
				dir,
				src,
				fmt.Sprintf("%s.%03d", dst, i),
				offset,
				size,
				fmt.Sprintf("%s (%d)", dst, i),
			)
			if err != nil {
				return nil, errors.Wrapf(err,
					"failed to open fragment %#v with offset %d and size %d",
					src, offset, size,
				)
			}
			files = append(files, *srcFile)
		}
	} else {
		size = fi.Size()
		offset = 0
		srcFile, err := open(ctx, dir, src, dst, offset, size, src)
		if err != nil {
			return nil, errors.Wrapf(err,
				"failed to open fragment %#v with offset %d and size %d",
				src, offset, size,
			)
		}
		files = append(files, *srcFile)
	}

	return files, nil
}

/*
func uploadFile(ctx *ctx, dir, src string, fi os.FileInfo, dst string) error {

	remotePaths, _, _ := getFileInfo(ctx, dst)

	for _, remotePath := range remotePaths {
		err := ctx.n.Delete(remotePath)
		if err != nil {
			return errors.Wrapf(err, "failed to delete %#v", remotePath)
		}
	}

	if 0 < ctx.splitSize && ctx.splitSize < fi.Size() {
		for i := int64(0); i*ctx.splitSize < fi.Size(); i++ {
			offset := i * ctx.splitSize
			var size int64
			if fi.Size() < (i+1)*ctx.splitSize {
				size = int64(fi.Size())
			} else {
				size = (i + 1) * ctx.splitSize
			}
			size -= i * ctx.splitSize
			uploadFragment(
				ctx,
				dir,
				src,
				offset,
				size,
				fmt.Sprintf("%s.%03d", dst, i),
				fmt.Sprintf("%s (%d)", dst, i),
			)
		}
		return nil
	}

	uploadFragment(ctx, dir, src, 0, fi.Size(), dst, src)

	return nil
}
*/

func uploadFragment(files []file) {

	for _, f := range files {

		f.ctx.sem <- struct{}{}
		f.ctx.wg.Add(1)
		go func(f file) {
			defer func() {
				f.ctx.wg.Done()
				<-f.ctx.sem
			}()

			var bar *pbpool.ProgressBar

			if f.ctx.pool == nil {
				fmt.Fprintln(os.Stdout, f.path)
			} else {
				bar = f.ctx.pool.Get()
				bar.SetTotal64(f.size)
				bar.Prefix(f.barPrefix)
				bar.SetUnits(pb.U_BYTES)
				bar.Start()
				bar.Set(0)
				defer func() {
					bar.Finish()
					f.ctx.pool.Put(bar)
				}()
			}
			f.bar = bar
			n := 0
			for {
				err := func() error {
					if err := f.ctx.n.WriteFile(f.dst, &f); err != nil {
						if !os.IsNotExist(err) {
							return errors.Wrapf(err,
								"failed to WriteFile fragment %#v with offset %d and size %d to %#v",
								f.path, f.offset, f.size, f.dst,
							)
						}
						if err := f.ctx.n.MkdirAll(f.dir); err != nil {
							return errors.Wrapf(err,
								"MkdirAll failed while handling non-existing file %#v",
								f.dst,
							)
						}

						if err := f.Reset(); err != nil {
							return errors.Wrapf(err,
								"Reset failed while handling non-existing file %#v",
								f.dst,
							)
						}

						if err := f.ctx.n.WriteFile(f.dst, &f); err != nil {
							return errors.Wrapf(err,
								"failed to retry WriteFile fragment %#v with offset %d and size %d to %#v",
								f.path, f.offset, f.size, f.dst,
							)
						}
					}
					return nil
				}()

				if err == nil {
					return
				}

				n++
				if f.ctx.retry > 0 && f.ctx.retry > n {
					fmt.Println("error! retry after " + f.ctx.delay.String() + "...")
					fmt.Println("  " + err.Error())
					time.Sleep(f.ctx.delay)
					continue
				}

				f.ctx.setError(
					errors.Wrapf(err,
						"failed %d times to upload fragment %#v with offset %d and size %d to %#v",
						f.ctx.retry, f.path, f.offset, f.size, f.dst,
					),
				)
				return
			}
		}(f)
	}
}

func open(ctx *ctx, dir string, path string, dst string, offset int64, size int64, barPrefix string) (*file, error) {
	rawfile, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %#v", path)
	}

	f := file{
		ctx:       ctx,
		File:      rawfile,
		dir:       dir,
		path:      path,
		dst:       dst,
		offset:    offset,
		size:      size,
		barPrefix: barPrefix,
		bar:       nil,
	}

	if _, err := f.File.Seek(offset, io.SeekStart); err != nil {
		return nil, errors.Wrapf(err, "failed to seek %#v to %d", f.path, offset)
	}

	return &f, nil
}

type file struct {
	*ctx
	*os.File
	dir       string
	path      string
	dst       string
	offset    int64
	size      int64
	barPrefix string
	bar       *pbpool.ProgressBar
}

func (f *file) Reset() error {

	if _, err := f.File.Seek(f.offset, io.SeekStart); err != nil {
		f.File.Close()

		rawfile, err := os.OpenFile(f.path, os.O_RDONLY, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to reopen %#v", f.path)
		}
		f.File = rawfile
		if _, err := f.File.Seek(f.offset, io.SeekStart); err != nil {
			return errors.Wrapf(err, "failed to re-seek %#v to %d", f.path, f.offset)
		}
	}

	if f.bar != nil {
		f.bar.Set(0)
	}

	return nil
}

func (f *file) Read(p []byte) (int, error) {
	var slice []byte
	cur, err := f.File.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get current offset of %#v", f.path)
	}
	remain := f.size - (cur - f.offset)
	if remain == 0 {
		return 0, io.EOF
	}
	if len(p) < int(remain) {
		slice = p
	} else {
		slice = p[0:int(remain)]
	}
	n, err := f.File.Read(slice)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %#v", f.path)
	}
	if f.bar != nil {
		f.bar.Add(n)
	}
	return n, nil
}
