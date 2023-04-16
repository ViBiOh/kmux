package concurrent

import (
	"sync"
)

type Simple struct {
	wg sync.WaitGroup
}

func NewSimple() *Simple {
	return &Simple{}
}

func (g *Simple) Go(f func()) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		f()
	}()
}

func (g *Simple) Wait() {
	g.wg.Wait()
}
