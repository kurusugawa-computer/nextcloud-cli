package ioutils

type ReaderFunc func([]byte) (int, error)

func (f ReaderFunc) Read(p []byte) (int, error) {
	return f(p)
}

type WriterFunc func([]byte) (int, error)

func (f WriterFunc) Write(p []byte) (int, error) {
	return f(p)
}

type CloserFunc func() error

func (f CloserFunc) Close() error {
	return f()
}

type SeekerFunc func(int64, int) (int64, error)

func (f SeekerFunc) Seek(offset int64, whence int) (int64, error) {
	return f(offset, whence)
}

type FlusherFunc func() error

func (f FlusherFunc) Flush() error {
	return f()
}
