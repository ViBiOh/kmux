package concurrent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
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

		defer func() {
			if r := recover(); r != nil {
				slog.LogAttrs(context.Background(), slog.LevelError, fmt.Sprintf("panic: %s", r), slog.String("error.stack", string(debug.Stack())))
			}
		}()

		f()
	}()
}

func (g *Simple) Wait() {
	g.wg.Wait()
}
