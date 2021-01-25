package progress

import (
	"math"
	"sync"
)

type ProgressWriter interface {
	SetTotal(int64)
	SetCurrent(int64)
}

type ProgressReader interface {
	GetTotalCurrent() (total, current int64)
	GetPercent() float64
}

type ProgressReadWriter interface {
	ProgressReader
	ProgressWriter
}

type ProgressComposite interface {
	ProgressReader
	Add(p ProgressReader)
}

func NewProgress() ProgressReadWriter {
	return &progress{}
}

type progress struct {
	total      int64
	totalMtx   sync.RWMutex
	current    int64
	currentMtx sync.RWMutex
}

func (p *progress) SetTotal(n int64) {
	p.totalMtx.Lock()
	defer p.totalMtx.Unlock()
	p.total = n
}

func (p *progress) SetCurrent(n int64) {
	p.currentMtx.Lock()
	defer p.currentMtx.Unlock()
	p.current = n
}

func (p *progress) GetTotalCurrent() (int64, int64) {
	p.totalMtx.RLock()
	defer p.totalMtx.RUnlock()
	p.currentMtx.RLock()
	defer p.currentMtx.RUnlock()
	return p.total, p.current
}

func (p *progress) GetPercent() float64 {
	total, current := p.GetTotalCurrent()
	percent := float64(current) / float64(total)
	if math.IsNaN(percent) {
		return 0
	}
	return percent
}

func NewComposite(parts ...ProgressReader) ProgressComposite {
	return &Composite{parts: parts}
}

type Composite struct {
	parts    []ProgressReader
	partsMtx sync.RWMutex
}

func (c *Composite) Add(p ProgressReader) {
	c.partsMtx.Lock()
	defer c.partsMtx.Unlock()
	c.parts = append(c.parts, p)
}

func (c *Composite) GetTotalCurrent() (int64, int64) {
	c.partsMtx.RLock()
	defer c.partsMtx.RUnlock()

	var total int64
	var current int64

	for _, p := range c.parts {
		t, c := p.GetTotalCurrent()
		total += t
		current += c
	}

	return total, current
}

func (c *Composite) GetPercent() float64 {
	total, current := c.GetTotalCurrent()
	percent := float64(current) / float64(total)
	if math.IsNaN(percent) {
		return 0
	}
	return percent
}
