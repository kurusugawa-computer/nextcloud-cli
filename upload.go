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
	"github.com/thamaji/ioutils"
	"gopkg.in/cheggaaa/pb.v1"
)

func upload(c *webdav.Client, src string, dst string, retry int) error {
	f, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
	}()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		p := path.Join(dst, stat.Name())

		switch DeconflictStrategy {
		case DeconflictSkip:
			if _, err := c.Stat(p); err == nil {
				fmt.Println("remote file already exists: skip " + p)
				return nil
			}

		case DeconflictOverwrite:

		case DeconflictNewest:
			if remote, err := c.Stat(p); err == nil {
				if !remote.ModTime().Before(stat.ModTime()) {
					fmt.Println("skip older file: " + p)
					return nil
				}
			}

		case DeconflictError:
			if _, err := c.Stat(p); err == nil {
				return errors.New("remote file already exists: " + p)
			}
		}

		n := 0
		for {
			err := uploadFile(c, src, f, stat, p)
			if err == nil {
				return nil
			}

			n++
			if retry > 0 && retry > n {
				fmt.Println("error! retry after 30 seconds...")
				time.Sleep(30 * time.Second)

				if _, err := f.Seek(0, io.SeekStart); err != nil {
					f.Close()
					fx, err := os.OpenFile(src, os.O_RDONLY, 0)
					if err != nil {
						return err
					}
					f = fx
					stat, err = f.Stat()
					if err != nil {
						return err
					}
				}

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

func uploadFile(c *webdav.Client, src string, f *os.File, stat os.FileInfo, p string) error {
	if stat.Size() > 100*1024*1024*1024 {
		fmt.Println("skip", src, stat.Size())
		return nil
	}

	if !stdoutIsTerminal {
		fmt.Fprintln(os.Stdout, src)
		return c.WriteStream(p, f, stat.Mode())
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

	if err := c.WriteStream(p, r, stat.Mode()); err != nil {
		return err
	}

	bar.Finish()

	return nil
}
