package client

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ViBiOh/kmux/pkg/output"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Kube struct {
	output.Outputter
	kubernetes.Interface
	Config    *rest.Config
	Name      string
	Namespace string
}

func New(name, namespace string, config *rest.Config, clientset kubernetes.Interface) Kube {
	return Kube{
		Outputter: output.NewOutputter(name),
		Interface: clientset,
		Config:    config,
		Name:      name,
		Namespace: namespace,
	}
}

type Action func(context.Context, Kube) error

type Array []Kube

type ReportedError struct {
	Err error
}

func (re ReportedError) Error() string {
	return re.Err.Error()
}

func (re ReportedError) Unwrap() error {
	return re.Err
}

// Execute prints each failure as it happens, the returned error is already reported.
func (a Array) Execute(ctx context.Context, action Action) error {
	var parallel sync.WaitGroup

	failures := make([]error, len(a))

	for index, client := range a {
		parallel.Go(func() {
			if err := action(ctx, client); err != nil {
				client.Err("%s", err)

				if len(client.Name) != 0 {
					err = fmt.Errorf("%s: %w", client.Name, err)
				}

				failures[index] = err
			}
		})
	}

	parallel.Wait()

	if err := errors.Join(failures...); err != nil {
		return ReportedError{Err: err}
	}

	return nil
}
