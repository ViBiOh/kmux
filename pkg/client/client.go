package client

import (
	"github.com/ViBiOh/kmux/pkg/concurrent"
	"github.com/ViBiOh/kmux/pkg/output"
	"k8s.io/client-go/kubernetes"
)

type Kube struct {
	output.Outputter
	*kubernetes.Clientset
	Name      string
	Namespace string
}

func New(name, namespace string, clientset *kubernetes.Clientset) Kube {
	return Kube{
		Outputter: output.NewOutputter(name),
		Clientset: clientset,
		Name:      name,
		Namespace: namespace,
	}
}

type Action func(Kube) error

type Array []Kube

func (a Array) Execute(action Action) {
	parallel := concurrent.NewSimple()

	for _, client := range a {
		client := client

		parallel.Go(func() {
			if err := action(client); err != nil {
				client.Err("%s", err)
			}
		})
	}

	parallel.Wait()
}
