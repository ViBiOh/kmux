package cmd

import (
	"github.com/ViBiOh/kube/pkg/output"
	"k8s.io/client-go/kubernetes"
)

type kubeClient struct {
	clientset *kubernetes.Clientset
	namespace string
}

type kubeAction func(string, kubeClient) error

func (kc kubeClient) execute(contextName string, action kubeAction) error {
	return action(contextName, kc)
}

type kubeClients map[string]kubeClient

func (kc kubeClients) execute(action kubeAction) {
	concurrent := newConcurrent()

	for contextName, client := range kc {
		contextName := contextName
		client := client

		concurrent.run(func() {
			if err := client.execute(contextName, action); err != nil {
				output.Err(contextName, "%s", err)
			}
		})
	}

	concurrent.wait()
}
