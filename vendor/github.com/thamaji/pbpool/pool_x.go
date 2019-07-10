// +build linux darwin freebsd netbsd openbsd solaris dragonfly

package pbpool

import (
	"bytes"
	"fmt"
)

func (p *Pool) Update() {
	p.m.Lock()

	buf := p.buffers.Get().(*bytes.Buffer)
	buf.Reset()

	if p.lines > 0 {
		fmt.Fprintf(buf, "\033[%dA", p.lines)
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
