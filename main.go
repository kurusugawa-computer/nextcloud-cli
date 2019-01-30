package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/skratchdot/open-golang/open"

	"github.com/dustin/go-humanize"
	"github.com/kurusugawa-computer/nextcloud-cli/fq"
	webdav "github.com/studio-b12/gowebdav"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/urfave/cli.v2"
)

var appname = "nextcloud-cli"

var stdinIsTerminal bool
var stdoutIsTerminal bool

var DeconflictStrategy int

const (
	DeconflictSkip = iota + 1
	DeconflictOverwrite
	DeconflictNewest
	DeconflictError
)

func parseDeconflictStrategy(s string) (int, error) {
	switch s {
	case "skip":
		return DeconflictSkip, nil
	case "overwrite":
		return DeconflictOverwrite, nil
	case "newest":
		return DeconflictNewest, nil
	case "error":
		return DeconflictError, nil
	default:
		return 0, errors.New("unknown deconflict strategy: " + s)
	}
}

func main() {
	stdinIsTerminal = terminal.IsTerminal(int(os.Stdin.Fd()))
	stdoutIsTerminal = terminal.IsTerminal(int(os.Stdout.Fd()))

	app := &cli.App{
		Name:      appname,
		Usage:     "NextCloud CLI",
		ArgsUsage: " ",
		Version:   "v1.0.8",
		Flags:     []cli.Flag{},
		Commands: []*cli.Command{
			&cli.Command{
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
						if !stdinIsTerminal {
							return errors.New("stdin is not a terminal")
						}

						fmt.Print("Enter username: ")
						if _, err := fmt.Fscanln(os.Stdin, &username); err != nil {
							return err
						}
					}

					password := ctx.String("password")
					if !ctx.IsSet("password") {
						if !stdinIsTerminal {
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

					credential := Credential{URL: u.String(), Username: username, Password: Password(password)}

					c := webdav.NewClient(credential.URL, credential.Username, credential.Password.String())
					c.Connect()
					if _, err := c.Stat("/"); err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					return SaveCredential(&Credential{URL: u.String(), Username: username, Password: Password(password)})
				},
			},
			&cli.Command{
				Name:        "logout",
				Usage:       "Logout from NextCloud",
				Description: "",
				ArgsUsage:   " ",
				Flags:       []cli.Flag{},
				Action: func(ctx *cli.Context) error {
					return Clean()
				},
			},
			&cli.Command{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List remote files or directories",
				Description: "",
				ArgsUsage:   "[FILE...]",
				Flags:       []cli.Flag{},
				Action: func(ctx *cli.Context) error {
					credential, err := LoadCredential()
					if err != nil {
						Clean()
						return errors.New("you need to login")
					}

					c, err := connect(credential)
					if err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					args := ctx.Args().Slice()

					if len(args) <= 0 {
						args = []string{"/"}
					}

					w := tabwriter.NewWriter(os.Stdout, 0, 4, 1, '\t', 0)

					type file struct {
						path string
						stat os.FileInfo
					}

					files := make([]*file, 0, len(args))
					for _, arg := range args {
						stat, err := c.Stat(arg)
						if err != nil {
							return err
						}
						files = append(files, &file{path: arg, stat: stat})
					}

					sort.Slice(files, func(i, j int) bool {
						if files[i].stat.IsDir() == files[j].stat.IsDir() {
							return files[i].path < files[j].path
						}
						return files[j].stat.IsDir()
					})

					for i, file := range files {
						if !file.stat.IsDir() {
							if _, err := fmt.Fprintln(w, formatFileInfo(file.stat, file.path)); err != nil {
								return err
							}
							continue
						}

						if len(files) > 1 {
							if err := w.Flush(); err != nil {
								return err
							}

							if i > 0 {
								if _, err := fmt.Fprintln(w); err != nil {
									return err
								}
							}

							if _, err := fmt.Fprintln(w, file.path+":"); err != nil {
								return err
							}
						}

						stats, err := c.ReadDir(file.path)
						if err != nil {
							return err
						}

						sort.Slice(stats, func(i, j int) bool {
							return stats[i].Name() < stats[j].Name()
						})

						for _, stat := range stats {
							if _, err := fmt.Fprintln(w, formatFileInfo(stat, stat.Name())); err != nil {
								return err
							}
						}
					}

					return w.Flush()
				},
			},
			&cli.Command{
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
					&cli.BoolFlag{
						Name:  "ls",
						Usage: "show files or directories in 'ls' style",
					},
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
					credential, err := LoadCredential()
					if err != nil {
						Clean()
						return errors.New("you need to login")
					}

					c, err := connect(credential)
					if err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					opts := findOptions{
						MaxDepth: ctx.Int("maxdepth"),
						MinDepth: ctx.Int("mindepth"),
						LS:       ctx.Bool("ls"),
					}

					args := ctx.Args().Slice()
					var files, tokens []string = args, nil
					for i := range args {
						if strings.HasPrefix(args[i], "-") || args[i] == "(" {
							files, tokens = args[:i], args[i:]
							break
						}
					}

					expr, err := fq.Parse(tokens...)
					if err != nil {
						return err
					}

					if len(files) <= 0 {
						files = []string{"."}
					}
					for _, file := range files {
						stat, err := c.Stat(file)
						if err != nil {
							return err
						}

						if err := find(c, &opts, 0, path.Clean(file), stat, expr); err != nil {
							return err
						}
					}

					return nil
				},
			},
			&cli.Command{
				Name:        "open",
				Usage:       "Open direcotries in your webbrowser",
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
					credential, err := LoadCredential()
					if err != nil {
						Clean()
						return errors.New("you need to login")
					}

					c, err := connect(credential)
					if err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					application := ctx.String("application")

					args := ctx.Args().Slice()

					if len(args) <= 0 {
						args = []string{"/"}
					}

					for _, arg := range args {
						stat, err := c.Stat(arg)
						if err != nil {
							return err
						}

						if !stat.IsDir() {
							return errors.New("path is not a directory: " + arg)
						}

						u, err := url.Parse(credential.URL)
						if err != nil {
							return err
						}
						u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/remote.php/webdav")
						u.Path = path.Join(u.Path, "/apps/files/")
						query := u.Query()
						query.Set("dir", arg)
						u.RawQuery = query.Encode()

						if application != "" {
							if err := open.StartWith(u.String(), application); err != nil {
								return err
							}
						} else {
							if err := open.Start(u.String()); err != nil {
								return err
							}
						}
					}

					return nil
				},
			},
			&cli.Command{
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
						Usage:   "set deconflict strategy (skip/overwrite/newest/error)",
						Value:   "error",
					},
				},
				Action: func(ctx *cli.Context) error {
					out := ctx.String("out")
					deconflict := ctx.String("deconflict")
					retry := ctx.Int("retry")

					if ctx.Args().Len() < 1 {
						return cli.ShowSubcommandHelp(ctx)
					}

					var err error
					DeconflictStrategy, err = parseDeconflictStrategy(deconflict)
					if err != nil {
						return err
					}

					credential, err := LoadCredential()
					if err != nil {
						Clean()
						return errors.New("you need to login")
					}

					c, err := connect(credential)
					if err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					for _, file := range ctx.Args().Slice() {
						if err := upload(c, file, out, retry); err != nil {
							return err
						}
					}

					return nil
				},
			},
			&cli.Command{
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
						Usage:   "set deconflict strategy (skip/overwrite/newest/error)",
						Value:   "error",
					},
				},
				Action: func(ctx *cli.Context) error {
					out := ctx.String("out")
					deconflict := ctx.String("deconflict")
					retry := ctx.Int("retry")

					if ctx.Args().Len() < 1 {
						return cli.ShowSubcommandHelp(ctx)
					}

					var err error
					DeconflictStrategy, err = parseDeconflictStrategy(deconflict)
					if err != nil {
						return err
					}

					credential, err := LoadCredential()
					if err != nil {
						Clean()
						return errors.New("you need to login")
					}

					c, err := connect(credential)
					if err != nil {
						return errors.New("failed to login NextCloud: " + credential.URL)
					}

					for _, file := range ctx.Args().Slice() {
						if err := download(c, file, out, retry); err != nil {
							return err
						}
					}

					return nil
				},
			},
		},
	}

	if tError := app.Run(os.Args); tError != nil {
		fmt.Fprintln(os.Stderr, tError.Error())
		os.Exit(1)
	}
}

func connect(credential *Credential) (*webdav.Client, error) {
	c := webdav.NewClient(credential.URL, credential.Username, credential.Password.String())
	c.Connect()

	// NextCloud のバグなのか、Connect() のエラーが当てにならない
	// 認証が通っているか確認するために、カラの Stat を実行
	_, err := c.Stat("/")
	return c, err
}

func formatFileInfo(stat os.FileInfo, name string) string {
	mode := stat.Mode().String()

	size := "   -"
	if stat.Size() > 0 {
		s := strings.Split(humanize.Bytes(uint64(stat.Size())), " ")
		size = fmt.Sprintf("%4s %s", s[0], s[1])
	}

	modTime := stat.ModTime().In(time.Local).Format("2006-01-02 15:04")

	if stat.IsDir() {
		name = strings.TrimSuffix(name, "/") + "/"
	}

	return mode + "\t" + size + "\t" + modTime + "\t" + name
}

type findOptions struct {
	MaxDepth int
	MinDepth int
	LS       bool
}

func find(c *webdav.Client, opts *findOptions, depth int, filePath string, stat os.FileInfo, expr fq.Expr) error {
	if opts.MaxDepth >= 0 && opts.MaxDepth < depth {
		return nil
	}

	if opts.MinDepth < 0 || opts.MinDepth <= depth {
		result, err := expr.Apply(filePath, stat)
		if err != nil {
			return err
		}

		if result {
			if opts.LS {
				fmt.Println(formatFileInfo(stat, filePath))
			} else {
				fmt.Println(filePath)
			}
		}
	}

	if !stat.IsDir() {
		return nil
	}

	fl, err := c.ReadDir(filePath)
	if err != nil {
		return err
	}

	for _, f := range fl {
		if err := find(c, opts, depth+1, path.Join(filePath, f.Name()), f, expr); err != nil {
			return err
		}
	}

	return nil
}
