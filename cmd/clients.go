package cmd

import "k8s.io/client-go/kubernetes"

type kubeClient struct {
	clientset *kubernetes.Clientset
	namespace string
}

func (kc kubeClient) execute(context string, action func(string, kubeClient)) {
	action(context, kc)
}

type kubeClients map[string]kubeClient

func (kc kubeClients) execute(action func(string, kubeClient)) {
	concurrent := newConcurrent()

	for context, client := range kc {
		context := context
		client := client

		concurrent.run(func() {
			client.execute(context, action)
		})
	}

	concurrent.wait()
}
