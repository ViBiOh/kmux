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

func (fs *forwardSet) add(pod types.UID) (chan struct{}, bool) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if _, ok := fs.forwards[pod]; ok {
		return nil, false
	}

	stopChan := make(chan struct{})
	fs.forwards[pod] = stopChan

	return stopChan, true
}

func (fs *forwardSet) stop(pod types.UID) {
	fs.mutex.Lock()
	stopChan, ok := fs.forwards[pod]
	delete(fs.forwards, pod)
	fs.mutex.Unlock()

	if ok {
		close(stopChan)
	}
}

func (fs *forwardSet) stopAll() {
	fs.mutex.Lock()
	forwards := fs.forwards
	fs.forwards = make(map[types.UID]chan struct{})
	fs.mutex.Unlock()

	for _, stopChan := range forwards {
		close(stopChan)
	}
}
