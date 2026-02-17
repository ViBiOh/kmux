package client

import (
	"context"
	"sync"

	"github.com/ViBiOh/kmux/pkg/output"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Kube struct {
	output.Outputter
	*kubernetes.Clientset
	Config    *rest.Config
	Name      string
	Namespace string
}

func New(name, namespace string, config *rest.Config, clientset *kubernetes.Clientset) Kube {
	return Kube{
		Outputter: output.NewOutputter(name),
		Clientset: clientset,
		Config:    config,
		Name:      name,
		Namespace: namespace,
	}
}

type Action func(context.Context, Kube) error

type Array []Kube

func (a Array) Execute(ctx context.Context, action Action) {
	var parallel sync.WaitGroup

	for _, client := range a {
		parallel.Go(func() {
			if err := action(ctx, client); err != nil {
				client.Err("%s", err)
			}
		})
	}

	parallel.Wait()
}
