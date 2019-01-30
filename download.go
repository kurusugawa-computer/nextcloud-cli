package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	webdav "github.com/studio-b12/gowebdav"
	"gopkg.in/cheggaaa/pb.v1"
)

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
