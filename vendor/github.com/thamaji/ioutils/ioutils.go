package ioutils

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Terminate is drop all and close
func Terminate(r io.ReadCloser) error {
	_, err := io.Copy(ioutil.Discard, r)
	if err1 := r.Close(); err == nil {
		err = err1
	}
	return err
}

func CreateFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err == nil {
		return f, nil
	}

	if !os.IsNotExist(err) {
		return nil, err
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
}

func WriteFile(path string, fn func(io.Writer) error) error {
	f, err := CreateFile(path)
	if err != nil {
		return err
	}

	err = fn(f)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return err
}

func ReadFile(path string, fn func(io.Reader) error) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	err = fn(f)
	f.Close()

	return err
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ReadDir(dir string) ([]os.FileInfo, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}

	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func ReadDirOrEmpty(dir string) []os.FileInfo {
	list, _ := ReadDir(dir)
	return list
}

func ReadDirNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}

	list, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func ReadDirNamesOrEmpty(dir string) []string {
	list, _ := ReadDirNames(dir)
	return list
}

func EachRegularFiles(dir string, fn func(os.FileInfo) error) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()

	for {
		fl, err := f.Readdir(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if !fl[0].Mode().IsRegular() {
			continue
		}

		if strings.HasPrefix(fl[0].Name(), ".") {
			continue
		}

		if err := fn(fl[0]); err != nil {
			return err
		}
	}

	return nil
}

func EachFiles(dir string, fn func(os.FileInfo) error) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()

	for {
		fl, err := f.Readdir(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if err := fn(fl[0]); err != nil {
			return err
		}
	}

	return nil
}

func EachLine(rd io.Reader, fn func([]byte) error) error {
	return EachLineSize(rd, 4096, fn)
}

func EachLineSize(rd io.Reader, size int, fn func([]byte) error) error {
	buf := make([]byte, 0, size)
	b := bufio.NewReaderSize(rd, cap(buf))
	for {
		line, isPrefix, err := b.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		buf = append(buf, line...)

		if isPrefix {
			continue
		}

		if err := fn(buf); err != nil {
			return err
		}

		buf = buf[:0]
	}

	return nil
}

func Move(oldpath, newpath string) error {
	if err := os.Rename(oldpath, newpath); err == nil {
		return nil
	}

	src, err := os.OpenFile(oldpath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	fi, err := src.Stat()
	if err != nil {
		src.Close()
		return err
	}

	if fi.IsDir() {
		if err := os.Mkdir(newpath, fi.Mode().Perm()); err != nil {
			src.Close()
			return err
		}

		fl, err := src.Readdir(-1)
		src.Close()
		if err != nil {
			return err
		}

		for _, fi := range fl {
			if err := Move(filepath.Join(oldpath, fi.Name()), filepath.Join(newpath, fi.Name())); err != nil {
				return err
			}
		}

		return os.Remove(oldpath)
	}

	dst, err := os.OpenFile(newpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		src.Close()
		return err
	}

	_, err = io.Copy(dst, src)
	src.Close()
	if err != nil {
		return err
	}

	if err := dst.Close(); err != nil {
		return err
	}

	return os.Remove(oldpath)
}

func Copy(oldpath, newpath string) error {
	src, err := os.OpenFile(oldpath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	fi, err := src.Stat()
	if err != nil {
		src.Close()
		return err
	}

	if fi.IsDir() {
		if err := os.Mkdir(newpath, fi.Mode().Perm()); err != nil {
			src.Close()
			return err
		}

		fl, err := src.Readdir(-1)
		src.Close()
		if err != nil {
			return err
		}

		for _, fi := range fl {
			if err := Copy(filepath.Join(oldpath, fi.Name()), filepath.Join(newpath, fi.Name())); err != nil {
				return err
			}
		}

		return nil
	}

	dst, err := os.OpenFile(newpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		src.Close()
		return err
	}

	_, err = io.Copy(dst, src)
	src.Close()
	if err != nil {
		return err
	}

	return dst.Close()
}
