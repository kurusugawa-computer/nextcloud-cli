package ioutils

import "io"

func NewReadCounter(r io.Reader) *ReadCounter {
	return &ReadCounter{r: r}
}

type ReadCounter struct {
	Count int64
	r     io.Reader
}

func (rc *ReadCounter) Read(p []byte) (int, error) {
	n, err := rc.r.Read(p)
	rc.Count += int64(n)
	return n, err
}

func NewWriteCounter(w io.Writer) *WriteCounter {
	return &WriteCounter{w: w}
}

type WriteCounter struct {
	Count int64
	w     io.Writer
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n, err := wc.w.Write(p)
	wc.Count += int64(n)
	return n, err
}
