package errgroup

import "sync"

type Func func() error

type ErrGroup interface {
	Add(Func)
	Wait() []error
}

func NewErrGroup() ErrGroup {
	return &errGroup{}
}

type errGroup struct {
	errors    []error
	errorsMtx sync.Mutex
	wg        sync.WaitGroup
}

func (g *errGroup) Add(fn Func) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.appendError(err)
		}
	}()
}

func (g *errGroup) Wait() []error {
	g.wg.Wait()
	return g.errors
}

func (g *errGroup) appendError(err error) {
	g.errorsMtx.Lock()
	defer g.errorsMtx.Unlock()
	g.errors = append(g.errors, err)
}
