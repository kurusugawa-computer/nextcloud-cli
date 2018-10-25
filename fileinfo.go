package main

import (
	"os"
	"time"
)

type fileInfo struct {
	name string
	stat os.FileInfo
}

func (fi fileInfo) Name() string {
	return fi.name
}

func (fi fileInfo) Size() int64 {
	return fi.stat.Size()
}

func (fi fileInfo) Mode() os.FileMode {
	return fi.stat.Mode()
}

func (fi fileInfo) ModTime() time.Time {
	return fi.stat.ModTime()
}

func (fi fileInfo) IsDir() bool {
	return fi.stat.IsDir()
}

func (fi fileInfo) Sys() interface{} {
	return fi.stat.Sys()
}
