package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/cmd/download"
	"github.com/kurusugawa-computer/nextcloud-cli/cmd/find"
	"github.com/kurusugawa-computer/nextcloud-cli/cmd/list"
	"github.com/kurusugawa-computer/nextcloud-cli/cmd/open"
	"github.com/kurusugawa-computer/nextcloud-cli/cmd/upload"
	"github.com/kurusugawa-computer/nextcloud-cli/credentials"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/webdav"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/singleflight"
	"gopkg.in/urfave/cli.v2"
)

var appname = "nextcloud-cli"

func main() {
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)

	defaultProcs := numCPU / 2
	if defaultProcs <= 0 {
		defaultProcs = 1
	}

	app := &cli.App{
		Name:                  appname,
		Usage:                 "NextCloud CLI",
		ArgsUsage:             " ",
		Version:               "v1.2.0",
		Flags:                 []cli.Flag{},
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:        "login",
				Usage:       "Login to NextCloud",
				Description: "",
				ArgsUsage:   "URL",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "username",
						Aliases: []string{"u"},
						Usage:   "set nextcloud username",
					},
					&cli.StringFlag{
						Name:    "password",
						Aliases: []string{"p"},
						Usage:   "set nextcloud password",
					},
				},
				Action: func(ctx *cli.Context) error {
					if ctx.Args().Len() <= 0 {
						return cli.ShowSubcommandHelp(ctx)
					}

					u, err := url.Parse(ctx.Args().First())
					if err != nil {
						return errors.New("invalid argument : url\n" + err.Error())
					}

					if !strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), "remote.php/webdav") {
						u.Path = path.Join(u.Path, "/remote.php/webdav/")
					}

					username := ctx.String("username")
					if !ctx.IsSet("username") {
						if !terminal.IsTerminal(int(os.Stdin.Fd())) {
							return errors.New("stdin is not a terminal")
						}

						fmt.Print("Enter username: ")
						if _, err := fmt.Fscanln(os.Stdin, &username); err != nil {
							return err
						}
					}

					password := ctx.String("password")
					if !ctx.IsSet("password") {
						if !terminal.IsTerminal(int(os.Stdin.Fd())) {
							return errors.New("stdin is not a terminal")
						}

						fmt.Print("Enter password: ")
						bytes, err := terminal.ReadPassword(int(os.Stdin.Fd()))
						fmt.Println()
						if err != nil {
							return err
						}

						password = string(bytes)
					}

					credential := credentials.Credential{
						URL:      u.String(),
						Username: username,
						Password: credentials.Password(password),
					}

					auth := webdav.BasicAuth(credential.Username, credential.Password.String())
					nextcloud := nextcloud.New(credential.URL, httpClient(), auth)

					if _, err := nextcloud.Stat("/"); err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					return credentials.Save(appname, &credential)
				},
			},
			{
				Name:        "logout",
				Usage:       "Logout from NextCloud",
				Description: "",
				ArgsUsage:   " ",
				Flags:       []cli.Flag{},
				Action: func(ctx *cli.Context) error {
					return credentials.Clean(appname)
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List remote files or directories",
				Description: "",
				ArgsUsage:   "[FILE...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "long",
						Aliases: []string{"l"},
						Usage:   "use a long listing format",
						Value:   false,
					},
				},
				Action: func(ctx *cli.Context) error {
					credential, err := credentials.Load(appname)
					if err != nil {
						credentials.Clean(appname)
						return errors.New("you need to login")
					}

					auth := webdav.BasicAuth(credential.Username, credential.Password.String())
					nextcloud := nextcloud.New(credential.URL, httpClient(), auth)

					args := ctx.Args().Slice()
					if len(args) <= 0 {
						args = []string{"/"}
					}

					return list.Do(nextcloud, ctx.Bool("long"), args...)
				},
			},
			{
				Name:        "find",
				Usage:       "Find remote files or directories",
				Description: "",
				ArgsUsage: `[FILE...] [EXPRESSION]

EXPRESSION
Operators
	( EXPR )	! EXPR	-not EXPR	EXPR1 -a EXPR2
	EXPR1 -and EXPR2	EXPR1 -o EXPR2	EXPR1 -or EXPR2

Tests
	-name PATTERN	-iname PATTERN	-path PATTERN	-ipath PATTERN
	-regex PATTERN	-mtime [-+]N	-newer FILE	-newermt YYYY-MM-dd
	-size [-+]N[kMG]	-empty	-type [fd]	-true	-false`,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "maxdepth",
						Usage: "set max descend levels",
						Value: -1,
					},
					&cli.IntFlag{
						Name:  "mindepth",
						Usage: "set min descend levels",
						Value: -1,
					},
				},
				Action: func(ctx *cli.Context) error {
					credential, err := credentials.Load(appname)
					if err != nil {
						credentials.Clean(appname)
						return errors.New("you need to login")
					}

					auth := webdav.BasicAuth(credential.Username, credential.Password.String())
					nextcloud := nextcloud.New(credential.URL, httpClient(), auth)

					args := ctx.Args().Slice()
					if len(args) <= 0 {
						args = []string{"/"}
					}

					var files, expressions []string = args, nil
					for i := range args {
						if strings.HasPrefix(args[i], "-") || args[i] == "(" || args[i] == "!" {
							files, expressions = args[:i], args[i:]
							break
						}
					}

					opts := []find.Option{
						find.MaxDepth(ctx.Int("maxdepth")),
						find.MinDepth(ctx.Int("mindepth")),
					}
					return find.Do(nextcloud, opts, files, expressions)
				},
			},
			{
				Name:        "open",
				Usage:       "Open remote files or directories in your webbrowser",
				Description: "",
				ArgsUsage:   "[Dir...]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "application",
						Usage:       "set webbrowser application",
						DefaultText: "default webbrowser",
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) <= 0 {
						args = []string{"/"}
					}

					credential, err := credentials.Load(appname)
					if err != nil {
						credentials.Clean(appname)
						return errors.New("you need to login")
					}

					auth := webdav.BasicAuth(credential.Username, credential.Password.String())
					nextcloud := nextcloud.New(credential.URL, httpClient(), auth)

					opts := []open.Option{
						open.AppName(ctx.String("application")),
					}
					return open.Do(nextcloud, opts, args)
				},
			},
			{
				Name:        "download",
				Usage:       "Download remote files or directories",
				Description: "",
				ArgsUsage:   "FILE [FILE...]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "out",
						Aliases: []string{"o"},
						Usage:   "set output directory",
						Value:   ".",
					},
					&cli.IntFlag{
						Name:    "retry",
						Aliases: []string{},
						Usage:   "set max retry count",
						Value:   5,
					},
					&cli.StringFlag{
						Name:    "deconflict",
						Aliases: []string{},
						Usage:   "set deconflict strategy (skip/overwrite/newest/larger/error)",
						Value:   "error",
					},
					&cli.IntFlag{
						Name:    "procs",
						Aliases: []string{},
						Usage:   "set maximum number of processes",
						Value:   defaultProcs,
					},
					&cli.BoolFlag{
						Name:    "join",
						Aliases: []string{},
						Usage:   "set true for automatic join",
						Value:   false,
					},
				},
				Action: func(ctx *cli.Context) error {
					if ctx.Args().Len() < 1 {
						return cli.ShowSubcommandHelp(ctx)
					}

					credential, err := credentials.Load(appname)
					if err != nil {
						credentials.Clean(appname)
						return errors.New("you need to login")
					}

					auth := webdav.BasicAuth(credential.Username, credential.Password.String())
					nextcloud := nextcloud.New(credential.URL, httpClient(), auth)

					opts := []download.Option{
						download.Retry(ctx.Int("retry"), 30*time.Second),
						download.DeconflictStrategy(ctx.String("deconflict")),
						download.Procs(ctx.Int("procs")),
						download.Join(ctx.Bool("join")),
					}
					return download.Do(nextcloud, opts, ctx.Args().Slice(), ctx.String("out"))
				},
			},
			{
				Name:        "upload",
				Usage:       "Upload local files or directories",
				Description: "",
				ArgsUsage:   "FILE [FILE...]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "out",
						Aliases: []string{"o"},
						Usage:   "set output directory",
						Value:   "/",
					},
					&cli.IntFlag{
						Name:    "retry",
						Aliases: []string{},
						Usage:   "set max retry count",
						Value:   5,
					},
					&cli.StringFlag{
						Name:    "deconflict",
						Aliases: []string{},
						Usage:   "set deconflict strategy (skip/overwrite/newest/larger/error)",
						Value:   "error",
					},
					&cli.IntFlag{
						Name:    "procs",
						Aliases: []string{},
						Usage:   "set maximum number of processes",
						Value:   defaultProcs,
					},
					&cli.StringFlag{
						Name:    "split-size",
						Aliases: []string{"s"},
						Usage:   "set splitting threshold",
						Value:   "",
					},
				},
				Action: func(ctx *cli.Context) error {
					if ctx.Args().Len() < 1 {
						return cli.ShowSubcommandHelp(ctx)
					}

					credential, err := credentials.Load(appname)
					if err != nil {
						credentials.Clean(appname)
						return errors.New("you need to login")
					}

					auth := webdav.BasicAuth(credential.Username, credential.Password.String())
					nextcloud := nextcloud.New(credential.URL, httpClient(), auth)

					opts := []upload.Option{
						upload.Retry(ctx.Int("retry"), 30*time.Second),
						upload.DeconflictStrategy(ctx.String("deconflict")),
						upload.Procs(ctx.Int("procs")),
						upload.SplitSize(ctx.String("split-size")),
					}
					return upload.Do(nextcloud, opts, ctx.Args().Slice(), ctx.String("out"))
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func httpClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}
	cache := sync.Map{}
	group := singleflight.Group{}

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
				i := strings.LastIndex(addr, ":")
				host := addr[:i]
				port := addr[i:]

				addrs, ok := cache.Load(host)
				if !ok {
					addrs, err, _ = group.Do(host, func() (interface{}, error) {
						return net.DefaultResolver.LookupHost(ctx, host)
					})
					if err == context.DeadlineExceeded {
						group.Forget(host)
						return
					}

					cache.Store(host, addrs)
				}

				for _, addr := range addrs.([]string) {
					conn, err = dialer.Dial(network, addr+port)
					if err == nil {
						return
					}
				}

				return
			},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
		},
	}
}
