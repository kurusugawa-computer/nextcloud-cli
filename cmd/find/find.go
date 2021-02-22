package find

import (
	"fmt"
	"os"
	_path "path"

	"github.com/kurusugawa-computer/nextcloud-cli/cmd/find/query"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
)

type Option func(*ctx) error

func MaxDepth(depth int) Option {
	return func(ctx *ctx) error {
		ctx.maxDepth = depth
		return nil
	}
}

func MinDepth(depth int) Option {
	return func(ctx *ctx) error {
		ctx.minDepth = depth
		return nil
	}
}

func Ls(b bool) Option {
	return func(ctx *ctx) error {
		ctx.ls = b
		return nil
	}
}

func Do(n *nextcloud.Nextcloud, opts []Option, paths []string, expressions []string) error {
	expr, err := query.Parse(expressions...)
	if err != nil {
		return err
	}

	ctx := &ctx{
		n: n,

		maxDepth: -1,
		minDepth: -1,
		ls:       false,
	}

	for _, opt := range opts {
		if err := opt(ctx); err != nil {
			return err
		}
	}

	for _, path := range paths {
		path := _path.Clean(path)

		fi, err := ctx.n.Stat(path)
		if err != nil {
			return err
		}

		if err := find(ctx, path, fi, expr, 0); err != nil {
			return err
		}
	}

	return nil
}

type ctx struct {
	n *nextcloud.Nextcloud

	maxDepth int
	minDepth int
	ls       bool
}

func find(ctx *ctx, path string, fi os.FileInfo, expr query.Expr, depth int) error {
	if ctx.maxDepth >= 0 && ctx.maxDepth < depth {
		return nil
	}

	if ctx.minDepth < 0 || ctx.minDepth <= depth {
		ok, err := expr.Apply(path, fi)
		if err != nil {
			return err
		}

		if ok {
			fmt.Println(path)
		}
	}

	if !fi.IsDir() {
		return nil
	}

	fl, err := ctx.n.ReadDir(path)
	if err != nil {
		return err
	}

	for _, fi := range fl {
		if err := find(ctx, _path.Join(path, fi.Name()), fi, expr, depth+1); err != nil {
			return err
		}
	}

	return nil
}
