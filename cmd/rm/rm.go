package rm

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"golang.org/x/crypto/ssh/terminal"
)

type Option func(*ctx) error

type ctx struct {
	n *nextcloud.Nextcloud // Nextcloud クライアント

	retry int           // リトライ回数
	delay time.Duration // リトライ時のディレイ

	recursive bool // ディレクトリとその中身を再帰的に削除
	force     bool // 操作の際に確認を取らない
	verbose   bool // 消したものを報告する
}

type ErrUserRefused struct{}

func (e *ErrUserRefused) Error() string {
	return "the user refused to delete"
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

func Recursive(b bool) Option {
	return func(ctx *ctx) error {
		ctx.recursive = b
		return nil
	}
}

func Force(b bool) Option {
	return func(ctx *ctx) error {
		ctx.force = b
		return nil
	}
}

func Verbose(b bool) Option {
	return func(ctx *ctx) error {
		ctx.verbose = b
		return nil
	}
}

func Do(n *nextcloud.Nextcloud, opts []Option, targets []string) error {
	ctx := &ctx{
		n:         n,
		retry:     3,
		delay:     30 * time.Second,
		recursive: false,
		force:     false,
	}

	for _, opt := range opts {
		if err := opt(ctx); err != nil {
			return err
		}
	}

	if !ctx.force && !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("stdin is not a terminal")
	}

	for _, target := range targets {
		if err := remove(ctx, target); err != nil {
			fmt.Printf("%v", err.Error())
		}
	}

	return nil
}

func askYesOrNo(format string, a ...interface{}) bool {
	fmt.Printf(format+" y/[n]: ", a...)
	var response string
	_, err := fmt.Fscanln(os.Stdin, &response)
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	if 0 < len(response) && response[0] == 'y' {
		return true
	}
	return false
}

func remove(ctx *ctx, target string) error {
	var fi fs.FileInfo
	var err error
	n := 0
	for {
		fi, err = ctx.n.Stat(target)
		if err == nil {
			break
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot remove '%v': %w", target, err)
		}
		n++
		if ctx.retry > 0 && ctx.retry > n {
			fmt.Println("error! retry after " + ctx.delay.String() + "...")
			fmt.Println("  " + err.Error())
			time.Sleep(ctx.delay)
			continue
		}
		return fmt.Errorf("cannot remove '%v': %w", target, err)
	}
	if fi.IsDir() {
		err = removeDir(ctx, target)
	} else {
		err = removeFile(ctx, target)
	}
	if err != nil && errors.Is(err, &ErrUserRefused{}) {
		return err
	}
	return nil
}

func retryReadDir(ctx *ctx, target string) ([]os.FileInfo, error) {
	var fis []fs.FileInfo
	var err error
	n := 0
	for {
		fis, err = ctx.n.ReadDir(target)
		if err == nil {
			return fis, nil
		}
		n++
		if ctx.retry > 0 && ctx.retry > n {
			fmt.Println("error! retry after " + ctx.delay.String() + "...")
			fmt.Println("  " + err.Error())
			time.Sleep(ctx.delay)
			continue
		}
		return nil, err
	}
}

func retryDelete(ctx *ctx, target string) error {
	n := 0
	for {
		err := ctx.n.Delete(target)
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

func removeDir(ctx *ctx, target string) error {
	if !ctx.recursive {
		fmt.Printf("cannot remove '%v': Is a directory\n", target)
		return nil
	}

	var fis []os.FileInfo
	var err error
	if fis, err = retryReadDir(ctx, target); err != nil {
		fmt.Printf("cannot remove '%v': %v\n", target, err.Error())
		return nil
	}

	remainingContentsCount := 0

	if len(fis) != 0 {
		if ctx.force || askYesOrNo("descend into directory '%v'?", target) {
			for _, fi := range fis {
				tTarget := path.Join(target, fi.Name())
				if fi.IsDir() {
					if err := removeDir(ctx, tTarget); err != nil {
						remainingContentsCount++
					}
				} else {
					if err := removeFile(ctx, tTarget); err != nil {
						remainingContentsCount++
					}
				}
			}
		}
	}

	if remainingContentsCount != 0 {
		return nil
	}

	if !(ctx.force || askYesOrNo("remove directory '%v'?", target)) {
		return &ErrUserRefused{}
	}

	if err := retryDelete(ctx, target); err != nil {
		return fmt.Errorf("cannot remove '%v': %w", target, err)
	}
	if ctx.verbose {
		fmt.Printf("removed directory '%v'\n", target)
	}
	return nil
}

func removeFile(ctx *ctx, target string) error {
	if !(ctx.force || askYesOrNo("remove file '%v'?", target)) {
		return &ErrUserRefused{}
	}
	if err := retryDelete(ctx, target); err != nil {
		return fmt.Errorf("cannot remove '%v': %w", target, err)
	}
	if ctx.verbose {
		fmt.Printf("removed '%v'\n", target)
	}
	return nil
}
