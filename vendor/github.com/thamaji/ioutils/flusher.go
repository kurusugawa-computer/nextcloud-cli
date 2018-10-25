package ioutils

type Flusher interface {
	Flush() error
}

var NopFlusher = FlusherFunc(func() error {
	return nil
})
