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

func (bp *Pool) Done() <-chan struct{} {
	return bp.done
}

func (bp *Pool) Add(backend string) *Pool {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	bp.backends = append(bp.backends, backend)

	return bp
}

func (bp *Pool) Remove(toRemove string) *Pool {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	index := slices.Index(bp.backends, toRemove)
	if index < 0 {
		return bp
	}

	bp.backends = slices.Delete(bp.backends, index, index+1)

	// keep the rotation on the same backend as before the deletion
	if uint64(index) < bp.current {
		bp.current--
	}

	return bp
}

// next returns the next backend in a round-robin fashion, empty when the pool
// has no backend.
func (bp *Pool) next() string {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if len(bp.backends) == 0 {
		return ""
	}

	backend := bp.backends[bp.current%uint64(len(bp.backends))]
	bp.current = (bp.current + 1) % uint64(len(bp.backends))

	return backend
}

func (bp *Pool) handle(upstream net.Conn) {
	backend := bp.next()
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

// Listen binds the local port, so a port already in use is reported before
// anything is forwarded.
func (bp *Pool) Listen(localPort uint64) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	bp.listener = listener

	return nil
}

// Serve accepts connections until the context is done, Listen must have
// succeeded first.
func (bp *Pool) Serve(ctx context.Context) {
	defer close(bp.done)

	if bp.listener == nil {
		output.Err("", "serve without a listener")

		return
	}

	go func() {
		for {
			conn, err := bp.listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}

				output.Err("", "listener accept: %s", err)

				continue
			}

			go bp.handle(conn)
		}
	}()

	<-ctx.Done()

	closeWithLog(bp.listener)
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
