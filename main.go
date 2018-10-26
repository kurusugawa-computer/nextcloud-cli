package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	webdav "github.com/studio-b12/gowebdav"
	"github.com/thamaji/ioutils"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/cheggaaa/pb.v1"
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
		Version:   "v1.0.5",
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

func download(c *webdav.Client, src string, dst string, retry int) error {
	stat, err := c.Stat(src)
	if err != nil {
		return err
	}

	if stat.Name() == "" {
		// nextcloud のバグなのか？ Name() が空文字になるので対応
		return _download(c, src, fileInfo{name: path.Base(src), stat: stat}, dst, retry)
	}

	return _download(c, src, stat, dst, retry)
}

func _download(c *webdav.Client, src string, stat os.FileInfo, dst string, retry int) error {
	if !stat.IsDir() {
		path := filepath.Join(dst, stat.Name())

		switch DeconflictStrategy {
		case DeconflictSkip:
			if _, err := os.Stat(path); err == nil {
				fmt.Println("skip older file: " + path)
				return nil
			}

		case DeconflictOverwrite:

		case DeconflictNewest:
			if remote, err := c.Stat(src); err == nil {
				if !stat.ModTime().Before(remote.ModTime()) {
					return nil
				}
			}

		case DeconflictError:
			if _, err := os.Stat(path); err == nil {
				return errors.New("local file already exists: " + path)
			}
		}

		n := 0
		for {
			err := downloadFile(c, src, stat, path)
			if err == nil {
				return nil
			}

			n++
			if retry > 0 && retry > n {
				fmt.Println("error! retry after 30 seconds...")
				time.Sleep(30 * time.Second)
				continue
			}

			return err
		}
	}

	dst = filepath.Join(dst, stat.Name())

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	fl, err := c.ReadDir(src)
	if err != nil {
		return err
	}

	for _, f := range fl {
		if err := _download(c, path.Join(src, f.Name()), f, dst, retry); err != nil {
			return err
		}
	}

	return nil
}

func downloadFile(c *webdav.Client, src string, stat os.FileInfo, path string) error {
	local, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	remote, err := c.ReadStream(src)
	if err != nil {
		local.Close()
		return err
	}

	if !stdoutIsTerminal {
		fmt.Fprintln(os.Stdout, src)

		_, err = io.Copy(local, remote)
		if err1 := local.Close(); err == nil {
			err = err1
		}
		remote.Close()

		return err
	}

	bar := pb.New(int(stat.Size()))
	bar.Prefix(src)
	bar.SetUnits(pb.U_BYTES)
	bar.Start()

	w := io.MultiWriter(local, bar)

	_, err = io.Copy(w, remote)
	if err1 := local.Close(); err == nil {
		err = err1
	}
	remote.Close()

	bar.Finish()

	return err
}

func uploadFile(c *webdav.Client, src string, f *os.File, stat os.FileInfo, path string) error {
	if !stdoutIsTerminal {
		fmt.Fprintln(os.Stdout, src)
		return c.WriteStream(path, f, stat.Mode())
	}

	bar := pb.New(int(stat.Size()))
	bar.Prefix(src)
	bar.SetUnits(pb.U_BYTES)
	bar.Start()

	r := ioutils.ReaderFunc(func(b []byte) (int, error) {
		n, err := f.Read(b)
		bar.Add(n)
		return n, err
	})

	if err := c.WriteStream(path, r, stat.Mode()); err != nil {
		return err
	}

	bar.Finish()

	return nil
}

func upload(c *webdav.Client, src string, dst string, retry int) error {
	f, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		path := path.Join(dst, stat.Name())

		switch DeconflictStrategy {
		case DeconflictSkip:
			if _, err := c.Stat(path); err == nil {
				fmt.Println("remote file already exists: skip " + path)
				return nil
			}

		case DeconflictOverwrite:

		case DeconflictNewest:
			if remote, err := c.Stat(path); err == nil {
				if !remote.ModTime().Before(stat.ModTime()) {
					fmt.Println("skip older file: " + path)
					return nil
				}
			}

		case DeconflictError:
			if _, err := c.Stat(path); err == nil {
				return errors.New("remote file already exists: " + path)
			}
		}

		n := 0
		for {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}

			err := uploadFile(c, src, f, stat, path)
			if err == nil {
				return nil
			}

			n++
			if retry > 0 && retry > n {
				fmt.Println("error! retry after 30 seconds...")
				time.Sleep(30 * time.Second)
				continue
			}

			return err
		}
	}

	dst = filepath.Join(dst, stat.Name())

	if err := c.MkdirAll(dst, stat.Mode()); err != nil {
		return err
	}

	for {
		fl, err := f.Readdir(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if err := upload(c, filepath.Join(src, fl[0].Name()), dst, retry); err != nil {
			return err
		}
	}

	return nil
}
