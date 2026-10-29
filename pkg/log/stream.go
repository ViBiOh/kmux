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

func (ss *streamSet) add(ctx context.Context, pod types.UID, container string) (context.Context, bool) {
	key := streamKey{pod: pod, container: container}

	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if _, ok := ss.streams[key]; ok {
		return nil, false
	}

	streamCtx, cancel := context.WithCancel(ctx)
	ss.streams[key] = cancel

	return streamCtx, true
}

func (ss *streamSet) remove(pod types.UID, container string) {
	key := streamKey{pod: pod, container: container}

	ss.mutex.Lock()
	cancel, ok := ss.streams[key]
	delete(ss.streams, key)
	ss.mutex.Unlock()

	if ok {
		cancel()
	}
}

func (ss *streamSet) cancelPod(pod types.UID) bool {
	ss.mutex.Lock()

	var cancels []context.CancelFunc

	for key, cancel := range ss.streams {
		if key.pod == pod {
			cancels = append(cancels, cancel)
			delete(ss.streams, key)
		}
	}

	ss.mutex.Unlock()

	for _, cancel := range cancels {
		cancel()
	}

	return len(cancels) > 0
}

func (ss *streamSet) markTerminal(pod types.UID) bool {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if ss.terminal[pod] {
		return false
	}

	ss.terminal[pod] = true

	return true
}

func (ss *streamSet) isTerminal(pod types.UID) bool {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	return ss.terminal[pod]
}
