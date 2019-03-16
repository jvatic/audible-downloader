package errgroup

import "sync"

// implementation of errgroup that records all errors without canceling tasks

// Func can return an error or []error
type Func func() interface{}

func NewGroup() *Group {
	return &Group{}
}

type Group struct {
	wg      sync.WaitGroup
	errsMtx sync.Mutex
	errs    []error
}

func (g *Group) Go(fn Func) {
	g.wg.Add(1)
	go (func() {
		defer g.wg.Done()

		err := fn()
		if err != nil {
			g.errsMtx.Lock()
			defer g.errsMtx.Unlock()
			if e, ok := err.(error); ok {
				g.errs = append(g.errs, e)
			} else if e, ok := err.([]error); ok {
				g.errs = append(g.errs, e...)
			}
		}
	})()
}

func (g *Group) Wait() []error {
	g.wg.Wait()
	g.errsMtx.Lock()
	defer g.errsMtx.Unlock()
	return g.errs
}
