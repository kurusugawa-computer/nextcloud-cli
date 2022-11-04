package get

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	_path "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/thamaji/pbpool"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/cheggaaa/pb.v1"
)

type ctx struct {
	n *nextcloud.Nextcloud // Nextcloud クライアント

	pool *pbpool.Pool // プログレスバーのプール

	deconflictStrategy int // ファイルが衝突したときの処理方法

	retry int           // リトライ回数
	delay time.Duration // リトライ時のディレイ

	join bool // 分割されていそうなファイルが存在したときに自動で結合するかどうか
}

type Option func(*ctx) error

const (
	DeconflictError     = "error"
	DeconflictOverwrite = "overwrite"
)

func DeconflictStrategy(strategy string) Option {
	return func(ctx *ctx) error {
		switch strategy {
		case DeconflictError:
			ctx.deconflictStrategy = 0

		case DeconflictOverwrite:
			ctx.deconflictStrategy = 1

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

func Join(b bool) Option {
	return func(ctx *ctx) error {
		ctx.join = b
		return nil
	}
}

func Do(n *nextcloud.Nextcloud, opts []Option, src string, dst string, rename string) error {
	ctx := &ctx{
		n: n,

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

	if err := download(ctx, src, dst, rename); err != nil {
		return err
	}

	if ctx.pool != nil {
		ctx.pool.Update()
		ctx.pool.Stop()
	}
	return nil
}

//file,directory,joinの場合分け
func download(ctx *ctx, src string, dst string, rename string) error {
	if ctx.join {
		// joinした後に同じsrcという名前になるものがないかチェックする
		fisMap, err := ctx.n.ReadJoinedDir(_path.Dir(src))
		if err != nil {
			return err
		}

		fls, ok := fisMap[_path.Base(src)]
		if !ok {
			fi, err := ctx.n.Stat(src)
			if err != nil {
				return err
			}
			return fmt.Errorf("unexpected: %s not found in %s by ReadJoinedDir, but actually exists", fi.Name(), _path.Dir(src))
		}

		if len(fls) != 1 {
			// joinした後に同じsrcという名前になるものが複数存在する
			names := []string{}
			for _, fis := range fls {
				for _, fi := range fis {
					names = append(names, fi.Name())
				}
			}
			return fmt.Errorf("name collision detected: %s", strings.Join(names, " "))
		}

		// joinした後にsrcとなるものがただ一つ存在する。
		if fls[0][0].IsDir() {
			tarFile, tarWriter, err := createTarFileAndWriter(ctx, src, dst, rename)
			if err != nil {
				return err
			}
			if err := downloadDir(ctx, src, dst, tarWriter); err != nil {
				return err
			}
			if err := tarWriter.Close(); err != nil {
				return err
			}
			if err := tarFile.Close(); err != nil {
				return err
			}
			return nil
		}

		srcs := []string{}
		for _, fi := range fls[0] {
			srcs = append(srcs, _path.Join(_path.Dir(src), fi.Name()))
		}

		return downloadAndJoinFiles(ctx, dst, srcs, filepath.Join(dst, rename))
	}

	fi, err := ctx.n.Stat(src)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return downloadFile(ctx, dst, src, filepath.Join(dst, rename))
	}

	tarFile, tarWriter, err := createTarFileAndWriter(ctx, src, dst, rename)
	if err != nil {
		return err
	}
	if err := downloadDir(ctx, src, dst, tarWriter); err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := tarFile.Close(); err != nil {
		return err
	}
	return nil
}

//tarFile,tarWriterを作成
func createTarFileAndWriter(ctx *ctx, src string, dst string, rename string) (*os.File, *tar.Writer, error) {
	switch ctx.deconflictStrategy {
	case 0: // DeconflictError
		if _, err := os.Stat(_path.Join(dst, rename)); err == nil {
			return nil, nil, fmt.Errorf("local file already exists: " + _path.Join(dst, rename))
		}
	case 1: // DeconflictOverwrite
	}

	fi, err := ctx.n.Stat(src)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(dst, fi.Mode()); err != nil {
		return nil, nil, err
	}

	tarFile, err := os.Create(_path.Join(dst, rename))
	return tarFile, tar.NewWriter(tarFile), err
}

//join以外からdownloadAndJoinFilesを呼ぶための関数
func downloadFile(ctx *ctx, dir string, src string, dst string) error {
	return downloadAndJoinFiles(ctx, dir, []string{src}, dst)
}

//ファイルダウンロード
func downloadAndJoinFiles(ctx *ctx, dir string, srcs []string, dst string) error {
	if len(srcs) == 0 {
		return errors.New("unexpected: tried to download empty file set")
	}

	switch ctx.deconflictStrategy {
	case 0: // DeconflictError
		if _, err := os.Stat(dst); err == nil {
			return errors.New("local file already exists: " + dst)
		}

	case 1: // DeconflictOverwrite
	}

	totalSize := int64(0)
	for _, src := range srcs {
		fi, err := ctx.n.Stat(src)
		if err != nil {
			return err
		}
		totalSize += fi.Size()
	}

	try := func() error {
		var bar *pbpool.ProgressBar

		if ctx.pool == nil {
			fmt.Fprintln(os.Stdout, dst)
		} else {
			bar = ctx.pool.Get()
			bar.SetTotal64(totalSize)
			bar.Prefix(dst)
			bar.SetUnits(pb.U_BYTES)
			bar.Start()
			defer func() {
				bar.Finish()
				ctx.pool.Put(bar)
			}()
		}

		srcFirstFileInfo, err := ctx.n.Stat(srcs[0])
		if err != nil {
			return err
		}

		if _, err := os.Stat(dir); err != nil {
			if err1 := os.MkdirAll(dir, 0775); err1 != nil {
				return err1
			}
		}
		dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcFirstFileInfo.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		var w io.Writer = dstFile
		if bar != nil {
			w = io.MultiWriter(dstFile, bar)
		}

		bw := bufio.NewWriter(w)

		for _, src := range srcs {
			srcFile, err := ctx.n.ReadFile(src)
			if err != nil {
				return err
			}

			if _, err := io.Copy(bw, srcFile); err != nil {
				srcFile.Close()
				return err
			}
			srcFile.Close()

			if err := bw.Flush(); err != nil {
				return err
			}

			if err := dstFile.Sync(); err != nil {
				return err
			}
		}
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

//ダウンロードするディレクトリ内のファイル一覧を作成して１つずつdownloadWithTarに渡す
func downloadDir(ctx *ctx, src string, dst string, tarWriter *tar.Writer) error {
	fi, err := ctx.n.Stat(src)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return fmt.Errorf("unexpected: %s is expected to directory", src)
	}

	type task struct {
		srcs []string
		name string
	}
	tasks := []*task{}

	if ctx.join {
		fisMap, err := ctx.n.ReadJoinedDir(src)
		if err != nil {
			return err
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
				return fmt.Errorf("name collision detected: %s", strings.Join(names, " "))
			}

			fl := fls[0]

			if fl[0].IsDir() {
				if err := downloadDir(ctx, _path.Join(src, fl[0].Name()), filepath.Join(dst, fl[0].Name()), tarWriter); err != nil {
					return err
				}
				continue
			}

			task := &task{
				srcs: []string{},
				name: name,
			}
			for _, fi := range fl {
				task.srcs = append(task.srcs, _path.Join(src, fi.Name()))
			}
			tasks = append(tasks, task)
		}

	} else {
		fl, err := ctx.n.ReadDir(src)
		if err != nil {
			return err
		}

		for _, fi := range fl {
			if fi.IsDir() {
				downloadDir(ctx, _path.Join(src, fi.Name()), filepath.Join(dst, fi.Name()), tarWriter)
				continue
			}
			tasks = append(tasks, &task{
				srcs: []string{_path.Join(src, fi.Name())},
				name: fi.Name(),
			})
		}
	}

	for _, task := range tasks {
		if err := downloadWithTar(ctx, task.srcs, task.name, tarWriter); err != nil {
			return err
		}
	}
	return nil
}

//tarWriterに書き込み
func downloadWithTar(ctx *ctx, srcs []string, fileName string, tarWriter *tar.Writer) error {
	if len(srcs) == 0 {
		return errors.New("unexpected: tried to download empty file set")
	}

	try := func() error {
		totalSize := int64(0)
		for _, src := range srcs {
			fi, err := ctx.n.Stat(src)
			if err != nil {
				return err
			}
			totalSize += fi.Size()
		}

		fi, err := ctx.n.Stat(srcs[0])
		if err != nil {
			return err
		}
		header := &tar.Header{
			Name:    _path.Join(_path.Dir(srcs[0]), fileName),
			Mode:    int64(fi.Mode()),
			ModTime: fi.ModTime(),
			Size:    totalSize,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		var bar *pbpool.ProgressBar

		if ctx.pool == nil {
			fmt.Fprintln(os.Stdout, _path.Join(_path.Dir(srcs[0]), fileName))
		} else {
			bar = ctx.pool.Get()
			bar.SetTotal64(totalSize)
			bar.Prefix(_path.Join(_path.Dir(srcs[0]), fileName))
			bar.SetUnits(pb.U_BYTES)
			bar.Start()
			defer func() {
				bar.Finish()
				ctx.pool.Put(bar)
			}()
		}

		var w io.Writer = tarWriter
		if bar != nil {
			w = io.MultiWriter(tarWriter, bar)
		}

		for _, src := range srcs {
			srcFile, err := ctx.n.ReadFile(src)
			if err != nil {
				return err
			}

			_, err = io.Copy(w, srcFile)

			srcFile.Close()
			if err != nil {
				return err
			}
		}
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
