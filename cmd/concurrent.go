package cmd

import "sync"

type Concurrent struct {
	wg sync.WaitGroup
}

func newConcurrent() *Concurrent {
	return &Concurrent{}
}

func (g *Concurrent) run(f func()) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		f()
	}()
}

func (g *Concurrent) wait() {
	g.wg.Wait()
}
