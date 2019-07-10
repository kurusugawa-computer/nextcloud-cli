// +build windows

package pbpool

import (
	"bytes"
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
var setConsoleCursorPosition = kernel32.NewProc("SetConsoleCursorPosition")

type smallRect struct {
	Left, Top, Right, Bottom int16
}

type coordinates struct {
	X, Y int16
}

type word int16

type consoleScreenBufferInfo struct {
	dwSize              coordinates
	dwCursorPosition    coordinates
	wAttributes         word
	srWindow            smallRect
	dwMaximumWindowSize coordinates
}

func getCursorPos() (pos coordinates, err error) {
	var info consoleScreenBufferInfo
	_, _, e := syscall.Syscall(procGetConsoleScreenBufferInfo.Addr(), 2, uintptr(syscall.Stdout), uintptr(unsafe.Pointer(&info)), 0)
	if e != 0 {
		return info.dwCursorPosition, error(e)
	}
	return info.dwCursorPosition, nil
}

func setCursorPos(pos coordinates) error {
	_, _, e := syscall.Syscall(setConsoleCursorPosition.Addr(), 2, uintptr(syscall.Stdout), uintptr(uint32(uint16(pos.Y))<<16|uint32(uint16(pos.X))), 0)
	if e != 0 {
		return error(e)
	}
	return nil
}

func (p *Pool) Update() {
	p.m.Lock()

	buf := p.buffers.Get().(*bytes.Buffer)
	buf.Reset()

	if p.lines > 0 {
		coords, err := getCursorPos()
		if err != nil {
			log.Panic(err)
		}
		coords.Y -= int16(p.lines)
		if coords.Y < 0 {
			coords.Y = 0
		}
		coords.X = 0

		err = setCursorPos(coords)
		if err != nil {
			log.Panic(err)
		}
	}

	for _, bar := range p.removed {
		fmt.Fprintf(buf, "\r%s\n", bar.String())
	}
	p.removed = p.removed[:0]

	for _, bar := range p.bars {
		bar.Update()
		fmt.Fprintf(buf, "\r%s\n", bar.String())
	}

	if p.Output != nil {
		fmt.Fprint(p.Output, buf.String())
	} else {
		fmt.Print(buf.String())
	}

	p.lines = len(p.bars)

	p.buffers.Put(buf)

	p.m.Unlock()
}
