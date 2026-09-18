package tcpool

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		backends []string
		want     []string
	}{
		"one backend": {
			[]string{"127.0.0.1:4000"},
			[]string{"127.0.0.1:4000"},
		},
		"keeps the insertion order": {
			[]string{"127.0.0.1:4000", "127.0.0.1:5000"},
			[]string{"127.0.0.1:4000", "127.0.0.1:5000"},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			instance := New()
			for _, backend := range testCase.backends {
				instance.Add(backend)
			}

			assert.Equal(t, testCase.want, instance.backends)
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		backends []string
		remove   string
		want     []string
	}{
		"empty pool": {
			nil,
			"127.0.0.1:4000",
			nil,
		},
		"unknown backend": {
			[]string{"127.0.0.1:4000"},
			"127.0.0.1:5000",
			[]string{"127.0.0.1:4000"},
		},
		"middle element": {
			[]string{"127.0.0.1:4000", "127.0.0.1:5000", "127.0.0.1:6000"},
			"127.0.0.1:5000",
			[]string{"127.0.0.1:4000", "127.0.0.1:6000"},
		},
		"last element": {
			[]string{"127.0.0.1:4000"},
			"127.0.0.1:4000",
			[]string{},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			instance := New()
			for _, backend := range testCase.backends {
				instance.Add(backend)
			}

			assert.Equal(t, testCase.want, instance.Remove(testCase.remove).backends)
		})
	}
}

func TestNext(t *testing.T) {
	t.Parallel()

	t.Run("empty pool", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, New().next())
	})

	t.Run("round robin", func(t *testing.T) {
		t.Parallel()

		instance := New().Add("a").Add("b").Add("c")

		assert.Equal(t, []string{"a", "b", "c", "a"}, []string{instance.next(), instance.next(), instance.next(), instance.next()})
	})

	t.Run("rotation is kept after a removal", func(t *testing.T) {
		t.Parallel()

		instance := New().Add("a").Add("b").Add("c")

		require.Equal(t, "a", instance.next())

		instance.Remove("a")

		assert.Equal(t, []string{"b", "c", "b"}, []string{instance.next(), instance.next(), instance.next()})
	})

	t.Run("removing the current backend", func(t *testing.T) {
		t.Parallel()

		instance := New().Add("a").Add("b")

		require.Equal(t, "a", instance.next())

		instance.Remove("b")

		assert.Equal(t, "a", instance.next(), "a single backend is always returned")
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	instance := New()

	var wg sync.WaitGroup

	for index := range 16 {
		wg.Go(func() {
			backend := fmt.Sprintf("127.0.0.1:%d", 4000+index)

			instance.Add(backend)
			instance.next()
			instance.Remove(backend)
		})
	}

	wg.Wait()

	assert.Empty(t, instance.backends)
}

func TestListenOnBusyPort(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, listener.Close())
	}()

	port := uint64(listener.Addr().(*net.TCPAddr).Port)

	err = New().Listen(port)

	assert.ErrorContains(t, err, "listen", "a port already in use must be reported")
}

// TestServe checks a connection is proxied to a backend.
func TestServe(t *testing.T) {
	t.Parallel()

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer func() {
		_ = backend.Close()
	}()

	go func() {
		conn, acceptErr := backend.Accept()
		if acceptErr != nil {
			return
		}

		defer func() {
			_ = conn.Close()
		}()

		_, _ = io.Copy(conn, conn)
	}()

	local, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	localPort := uint64(local.Addr().(*net.TCPAddr).Port)
	require.NoError(t, local.Close())

	instance := New().Add(backend.Addr().String())
	require.NoError(t, instance.Listen(localPort))

	ctx, cancel := context.WithCancel(context.Background())

	go instance.Serve(ctx)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), time.Second)
	require.NoError(t, err)

	_, err = conn.Write([]byte("ping\n"))
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))

	payload := make([]byte, 5)
	_, err = io.ReadFull(conn, payload)
	require.NoError(t, err)

	assert.Equal(t, "ping\n", string(payload))

	require.NoError(t, conn.Close())

	cancel()

	select {
	case <-instance.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("pool did not stop")
	}
}

func TestServeWithoutBackend(t *testing.T) {
	t.Parallel()

	local, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	localPort := uint64(local.Addr().(*net.TCPAddr).Port)
	require.NoError(t, local.Close())

	instance := New()
	require.NoError(t, instance.Listen(localPort))

	ctx, cancel := context.WithCancel(context.Background())

	go instance.Serve(ctx)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), time.Second)
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))

	// the connection is closed right away, no backend can serve it
	_, err = io.ReadAll(conn)
	assert.NoError(t, err)

	cancel()
	<-instance.Done()
}
