package tcpool

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/ViBiOh/kmux/pkg/output"
)

type Pool struct {
	sync.RWMutex
	backends []string
	current  uint64
	done     chan struct{}
}

func New() *Pool {
	return &Pool{
		done: make(chan struct{}),
	}
}

func (bp *Pool) Done() <-chan struct{} {
	return bp.done
}

func (bp *Pool) Add(backend string) {
	bp.Lock()
	defer bp.Unlock()

	bp.backends = append(bp.backends, backend)
}

func (bp *Pool) Remove(toRemove string) {
	bp.Lock()
	defer bp.Unlock()

	backends := bp.backends[:0]
	for _, backend := range bp.backends {
		if backend == toRemove {
			continue
		}

		backends = append(backends, backend)
	}

	bp.backends = backends
}

func (bp *Pool) next() string {
	bp.Lock()
	defer bp.Unlock()

	backendsLen := uint64(len(bp.backends))
	if backendsLen == 0 {
		return ""
	}

	bp.current = (bp.current + 1) % backendsLen

	return bp.backends[bp.current]
}

func (bp *Pool) handle(us net.Conn, server string) {
	ds, err := net.Dial("tcp", server)
	if err != nil {
		us.Close()
		output.Err("", "dial %s: %s", server, err)
		return
	}

	go copy(ds, us)
	go copy(us, ds)
}

func (bp *Pool) Start(ctx context.Context, localPort uint64) {
	listenerConfig := &net.ListenConfig{}
	listener, err := listenerConfig.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		output.Err("", "listen: %s", err)
		return
	}

	defer close(bp.done)

	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			output.Err("", "listener close: %s", closeErr)
		}
	}()

	output.Std("", "Listening tcp on %d", localPort)
	defer output.Std("", "Listening ended.")

	connChan := make(chan net.Conn, 4)
	go func() {
		defer close(connChan)

		for {
			conn, err := listener.Accept()
			if err != nil {
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					return
				}

				output.Err("", "listener accept: %s", err)
				continue
			}

			connChan <- conn
		}
	}()

	for {
		select {
		case conn := <-connChan:
			go bp.handle(conn, bp.next())
		case <-ctx.Done():
			return
		}
	}
}

func copy(wc io.WriteCloser, r io.Reader) {
	defer wc.Close()
	if _, err := io.Copy(wc, r); err != nil {
		output.Err("", "pool copy: %s", err)
	}
}
