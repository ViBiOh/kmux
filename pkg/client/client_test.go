package client

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "default"

func TestExecute(t *testing.T) {
	t.Parallel()

	instance := Array{
		New("one", testNamespace, nil, fake.NewClientset()),
		New("two", testNamespace, nil, fake.NewClientset()),
	}

	var seen []string

	var mutex sync.Mutex

	instance.Execute(context.Background(), func(_ context.Context, kube Kube) error {
		mutex.Lock()
		defer mutex.Unlock()

		seen = append(seen, kube.Name)

		return nil
	})

	assert.ElementsMatch(t, []string{"one", "two"}, seen)
}

func TestExecuteReportsError(t *testing.T) {
	t.Parallel()

	instance := Array{New("one", testNamespace, nil, fake.NewClientset())}

	assert.NotPanics(t, func() {
		instance.Execute(context.Background(), func(_ context.Context, _ Kube) error {
			return assert.AnError
		})
	}, "a failing action is reported, not propagated")
}
