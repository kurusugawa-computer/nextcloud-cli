package pbpool

import (
	"bytes"
	"io"
	"sync"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
)

func New() *Pool {
	return &Pool{
		buffers: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

type Pool struct {
	Output      io.Writer
	RefreshRate time.Duration
	m           sync.Mutex
	nextID      int
	bars        []*ProgressBar
	removed     []*ProgressBar
	lines       int
	buffers     sync.Pool
	shutdownCh  chan struct{}
}

type ProgressBar struct {
	id int
	*pb.ProgressBar
}

func (p *Pool) Get() *ProgressBar {
	p.m.Lock()

	bar := &ProgressBar{
		id: p.nextID,
		ProgressBar: func() *pb.ProgressBar {
			bar := pb.New(0)
			bar.ManualUpdate = true
			bar.NotPrint = true
			return bar
		}(),
	}
	p.bars = append(p.bars, bar)
	p.nextID++

	p.m.Unlock()

	return bar
}

func (p *Pool) Put(bar *ProgressBar) {
	p.m.Lock()

	bars := make([]*ProgressBar, 0, len(p.bars))

	for _, bar1 := range p.bars {
		if bar1.id == bar.id {
			p.removed = append(p.removed, bar1)
			continue
		}

		bars = append(bars, bar1)
	}

	p.bars = bars

	p.m.Unlock()
}

func (p *Pool) Start() {
	p.RefreshRate = pb.DEFAULT_REFRESH_RATE
	p.shutdownCh = make(chan struct{})

	go func() {
		p.Update()

		for {
			select {
			case <-time.After(p.RefreshRate):
				p.Update()

			case <-p.shutdownCh:
				return
			}
		}
	}()
}

func (p *Pool) Stop() {
	close(p.shutdownCh)
}
