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
	done     chan struct{}
	backends []string
	current  uint64
	mutex    sync.Mutex
}

func New() *Pool {
	return &Pool{}
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

	backends := bp.backends[:0]
	for _, backend := range bp.backends {
		if backend == toRemove {
			continue
		}

		backends = append(backends, backend)
	}

	bp.backends = backends

	return bp
}

func (bp *Pool) next() string {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

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
		output.Err("", "dial %s: %s", server, err)

		if closeErr := us.Close(); closeErr != nil {
			output.Err("", "close error: %s", closeErr)
		}

		return
	}

	go stream(ds, us)
	go stream(us, ds)
}

func (bp *Pool) Start(ctx context.Context, localPort uint64) {
	bp.done = make(chan struct{})
	defer close(bp.done)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		output.Err("", "listen: %s", err)
		return
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					return
				}

				output.Err("", "listener accept: %s", err)
				continue
			}

			go bp.handle(conn, bp.next())
		}
	}()

	<-ctx.Done()

	if closeErr := listener.Close(); closeErr != nil {
		output.Err("", "listener close: %s", closeErr)
	}
}

func stream(writer io.WriteCloser, reader io.Reader) {
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			output.Err("", "close error: %s", closeErr)
		}
	}()

	if _, err := io.Copy(writer, reader); err != nil {
		if !strings.HasSuffix(err.Error(), "use of closed network connection") {
			output.Err("", "pool copy: %s", err)
		}
	}
}
