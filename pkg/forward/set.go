package forward

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

type forwardSet struct {
	forwards map[types.UID]chan struct{}
	mutex    sync.Mutex
}

func newForwardSet() *forwardSet {
	return &forwardSet{
		forwards: make(map[types.UID]chan struct{}),
	}
}

func (f *forwardSet) add(pod types.UID) (chan struct{}, bool) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if _, ok := f.forwards[pod]; ok {
		return nil, false
	}

	stopChan := make(chan struct{})
	f.forwards[pod] = stopChan

	return stopChan, true
}

func (f *forwardSet) stop(pod types.UID) {
	f.mutex.Lock()
	stopChan, ok := f.forwards[pod]
	delete(f.forwards, pod)
	f.mutex.Unlock()

	if ok {
		close(stopChan)
	}
}

func (f *forwardSet) stopAll() {
	f.mutex.Lock()
	forwards := f.forwards
	f.forwards = make(map[types.UID]chan struct{})
	f.mutex.Unlock()

	for _, stopChan := range forwards {
		close(stopChan)
	}
}
