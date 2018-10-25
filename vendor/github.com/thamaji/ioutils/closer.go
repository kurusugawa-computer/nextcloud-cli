package ioutils

import (
	"io"
)

var NopCloser = CloserFunc(func() error {
	return nil
})

func NewReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return &ReadCloser{Reader: r, Closer: c}
}

type ReadCloser struct {
	io.Reader
	io.Closer
}

func NewWriteCloser(w io.Writer, c io.Closer) io.WriteCloser {
	return &WriteCloser{Writer: w, Closer: c}
}

type WriteCloser struct {
	io.Writer
	io.Closer
}

func NewReadWriteCloser(r io.Reader, w io.Writer, c io.Closer) io.ReadWriteCloser {
	return &ReadWriteCloser{Reader: r, Writer: w, Closer: c}
}

type ReadWriteCloser struct {
	io.Reader
	io.Writer
	io.Closer
}
