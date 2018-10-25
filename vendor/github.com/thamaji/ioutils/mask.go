package ioutils

import (
	"bytes"
	"io"
)

func NewMaskWriter(w io.Writer, secret []byte, mask byte) *MaskWriter {
	m := make([]byte, len(secret))
	for i := 0; i < len(m); i++ {
		m[i] = mask
	}
	return &MaskWriter{w: w, secret: secret, mask: m}
}

type MaskWriter struct {
	w       io.Writer
	secret  []byte
	mask    []byte
	matched int
}

func (mw *MaskWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	if len(mw.secret) == 0 {
		return mw.w.Write(p)
	}

	var start, end, checkpoint, t int

	if mw.matched > 0 {
		t = len(mw.secret) - mw.matched
		if t < len(p) && bytes.Equal(p[:t], mw.secret[mw.matched:]) {
			start, checkpoint = t, t
			n, err = mw.w.Write(mw.mask)
			if err != nil {
				return
			}
		}
	}

	for start < len(p)-len(mw.secret) {
		end = start + len(mw.secret)
		if !bytes.Equal(p[start:end], mw.secret) {
			start++
			continue
		}

		t, err = mw.w.Write(p[checkpoint:start])
		if err != nil {
			return
		}
		n += t

		t, err = mw.w.Write(mw.mask)
		if err != nil {
			return
		}
		n += t

		start, checkpoint = end, end
	}

	for t = len(mw.secret) - 1; t > 0; t-- {
		if t < len(p) && bytes.Equal(p[len(p)-t:], mw.secret[:t]) {
			mw.matched = t
			break
		}
	}

	t, err = mw.w.Write(p[checkpoint:])
	if err != nil {
		return
	}
	n += t

	return
}
