package tcpool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/ViBiOh/kmux/pkg/output"
)

const dialTimeout = 10 * time.Second

type Pool struct {
	done     chan struct{}
	listener net.Listener
	backends []string
	current  uint64
	mutex    sync.Mutex
}

func New() *Pool {
	return &Pool{
		done: make(chan struct{}),
	}
}

func (p *Pool) Done() <-chan struct{} {
	return p.done
}

func (p *Pool) Add(backend string) *Pool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.backends = append(p.backends, backend)

	return p
}

func (p *Pool) Remove(toRemove string) *Pool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	index := slices.Index(p.backends, toRemove)
	if index < 0 {
		return p
	}

	p.backends = slices.Delete(p.backends, index, index+1)

	if uint64(index) < p.current {
		p.current--
	}

	return p
}

func (p *Pool) next() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.backends) == 0 {
		return ""
	}

	backend := p.backends[p.current%uint64(len(p.backends))]
	p.current = (p.current + 1) % uint64(len(p.backends))

	return backend
}

func (p *Pool) handle(ctx context.Context, upstream net.Conn) {
	backend := p.next()
	if len(backend) == 0 {
		output.Err("", "no pod available to forward to")

		closeWithLog(upstream)

		return
	}

	downstream, err := (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, "tcp", backend)
	if err != nil {
		output.Err("", "dial %s: %s", backend, err)

		closeWithLog(upstream)

		return
	}

	pipe(ctx, upstream, downstream)
}

func (p *Pool) Listen(localPort uint64) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	p.listener = listener

	return nil
}

func (p *Pool) Serve(ctx context.Context) {
	defer close(p.done)

	if p.listener == nil {
		output.Err("", "serve without a listener")

		return
	}

	var serving sync.WaitGroup

	serving.Go(func() {
		for {
			conn, err := p.listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}

				output.Err("", "listener accept: %s", err)

				continue
			}

			serving.Go(func() {
				p.handle(ctx, conn)
			})
		}
	})

	<-ctx.Done()

	closeWithLog(p.listener)

	serving.Wait()
}

func pipe(ctx context.Context, upstream, downstream net.Conn) {
	closeBoth := func() {
		closeWithLog(upstream)
		closeWithLog(downstream)
	}

	defer closeBoth()
	defer context.AfterFunc(ctx, closeBoth)()

	var copying sync.WaitGroup

	copying.Go(func() { stream(downstream, upstream, closeBoth) })
	copying.Go(func() { stream(upstream, downstream, closeBoth) })

	copying.Wait()
}

// stream ends the writing side only on a clean end of input, so a peer that stops sending still receives the answer.
func stream(writer net.Conn, reader io.Reader, abort func()) {
	if _, err := io.Copy(writer, reader); err != nil {
		if !errors.Is(err, net.ErrClosed) {
			output.Err("", "pool copy: %s", err)
		}

		abort()

		return
	}

	if halfCloser, ok := writer.(interface{ CloseWrite() error }); ok {
		if err := halfCloser.CloseWrite(); err != nil && !errors.Is(err, net.ErrClosed) {
			output.Err("", "close write: %s", err)
		}

		return
	}

	closeWithLog(writer)
}

func closeWithLog(closer io.Closer) {
	if err := closer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		output.Err("", "close: %s", err)
	}
}
