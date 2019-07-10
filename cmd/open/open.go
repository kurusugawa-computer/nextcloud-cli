package open

import (
	"net/url"
	"os"
	_path "path"
	"strings"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	_open "github.com/skratchdot/open-golang/open"
)

type Option func(*ctx) error

func AppName(appName string) Option {
	return func(ctx *ctx) error {
		ctx.appName = appName
		return nil
	}
}

func Do(n *nextcloud.Nextcloud, opts []Option, paths []string) error {
	ctx := &ctx{
		n: n,

		appName: "",
	}

	for _, opt := range opts {
		if err := opt(ctx); err != nil {
			return err
		}
	}

	for _, path := range paths {
		if err := open(ctx, path); err != nil {
			return err
		}
	}

	return nil
}

type ctx struct {
	n *nextcloud.Nextcloud

	appName string
}

func open(ctx *ctx, path string) error {
	fi, err := ctx.n.Stat(path)
	if err != nil {
		return err
	}

	nfi, ok := fi.(*nextcloud.FileInfo)
	if !ok {
		return &os.PathError{Op: "Open", Path: path, Err: os.ErrInvalid}
	}

	u, err := url.Parse(ctx.n.URL)
	if err != nil {
		return err
	}
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/remote.php/webdav")
	u.Path = _path.Join(u.Path, "/apps/files/")
	query := u.Query()
	query.Set("fileid", nfi.ID())
	u.RawQuery = query.Encode()

	if ctx.appName == "" {
		return _open.Start(u.String())
	}
	return _open.StartWith(u.String(), ctx.appName)

}
