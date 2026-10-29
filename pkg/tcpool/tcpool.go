package tcpool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"

	"github.com/ViBiOh/kmux/pkg/output"
)

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

func (p *Pool) handle(upstream net.Conn) {
	backend := p.next()
	if len(backend) == 0 {
		output.Err("", "no pod available to forward to")

		closeWithLog(upstream)

		return
	}

	downstream, err := net.Dial("tcp", backend)
	if err != nil {
		output.Err("", "dial %s: %s", backend, err)

		closeWithLog(upstream)

		return
	}

	go stream(downstream, upstream)
	go stream(upstream, downstream)
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

	go func() {
		for {
			conn, err := p.listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}

				output.Err("", "listener accept: %s", err)

				continue
			}

			go p.handle(conn)
		}
	}()

	<-ctx.Done()

	closeWithLog(p.listener)
}

func stream(writer io.WriteCloser, reader io.Reader) {
	defer closeWithLog(writer)

	if _, err := io.Copy(writer, reader); err != nil && !errors.Is(err, net.ErrClosed) {
		output.Err("", "pool copy: %s", err)
	}
}

func closeWithLog(closer io.Closer) {
	if err := closer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		output.Err("", "close: %s", err)
	}
}
