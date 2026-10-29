package client

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	err := instance.Execute(context.Background(), func(_ context.Context, kube Kube) error {
		mutex.Lock()
		defer mutex.Unlock()

		seen = append(seen, kube.Name)

		return nil
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"one", "two"}, seen)
}

func TestExecuteReturnsJoinedError(t *testing.T) {
	t.Parallel()

	errOne := errors.New("first failure")
	errTwo := errors.New("second failure")

	instance := Array{
		New("one", testNamespace, nil, fake.NewClientset()),
		New("two", testNamespace, nil, fake.NewClientset()),
		New("three", testNamespace, nil, fake.NewClientset()),
	}

	err := instance.Execute(context.Background(), func(_ context.Context, kube Kube) error {
		switch kube.Name {
		case "one":
			return errOne

		case "two":
			return errTwo

		default:
			return nil
		}
	})

	require.Error(t, err)
	require.ErrorIs(t, err, errOne)
	require.ErrorIs(t, err, errTwo)
	assert.Contains(t, err.Error(), "one: first failure")
	assert.Contains(t, err.Error(), "two: second failure")
	assert.NotContains(t, err.Error(), "three")

	var reported ReportedError

	assert.ErrorAs(t, err, &reported)
}
