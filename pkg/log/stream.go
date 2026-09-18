package log

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

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
