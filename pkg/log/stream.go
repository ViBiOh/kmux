package log

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// streamSet tracks the running streams, one per pod's container. Registration
// happens before the stream starts so two events for the same pod cannot open
// the same stream twice.
type streamSet struct {
	streams  map[streamKey]context.CancelFunc
	terminal map[types.UID]bool
	mutex    sync.Mutex
}

type streamKey struct {
	pod       types.UID
	container string
}

func newStreamSet() *streamSet {
	return &streamSet{
		streams:  make(map[streamKey]context.CancelFunc),
		terminal: make(map[types.UID]bool),
	}
}

// add registers a stream and returns its context, false when it is already running.
func (s *streamSet) add(ctx context.Context, pod types.UID, container string) (context.Context, bool) {
	key := streamKey{pod: pod, container: container}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.streams[key]; ok {
		return nil, false
	}

	streamCtx, cancel := context.WithCancel(ctx)
	s.streams[key] = cancel

	return streamCtx, true
}

func (s *streamSet) remove(pod types.UID, container string) {
	key := streamKey{pod: pod, container: container}

	s.mutex.Lock()
	cancel, ok := s.streams[key]
	delete(s.streams, key)
	s.mutex.Unlock()

	if ok {
		cancel()
	}
}

// cancelPod stops every stream of a pod and tells whether there was any.
func (s *streamSet) cancelPod(pod types.UID) bool {
	s.mutex.Lock()

	var cancels []context.CancelFunc

	for key, cancel := range s.streams {
		if key.pod == pod {
			cancels = append(cancels, cancel)
			delete(s.streams, key)
		}
	}

	s.mutex.Unlock()

	for _, cancel := range cancels {
		cancel()
	}

	return len(cancels) > 0
}

// markTerminal flags a pod as done and returns false if it already was.
func (s *streamSet) markTerminal(pod types.UID) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.terminal[pod] {
		return false
	}

	s.terminal[pod] = true

	return true
}

func (s *streamSet) isTerminal(pod types.UID) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.terminal[pod]
}
