package concurrent

import (
	"sync"
)

// Simple describes a task group with simple parallelism.
type Simple struct {
	wg sync.WaitGroup
}

func NewSimple() *Simple {
	return &Simple{}
}

// Go run given function in a goroutine.
func (g *Simple) Go(f func()) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		f()
	}()
}

// Wait for Simple to end.
func (g *Simple) Wait() {
	g.wg.Wait()
}
