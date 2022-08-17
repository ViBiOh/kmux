package cmd

import (
	"context"
	"sort"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getCommonNamespace(kube client.Kube, namespace string) string {
	if len(namespace) != 0 {
		return namespace
	}

	if !allNamespace {
		return kube.Namespace
	}

	return ""
}

func getCommonObjects(namespace string, lister resource.Lister) []string {
	output := make(chan string, len(clients))
	successChan := make(chan bool, len(clients))

	go func() {
		defer close(output)
		defer close(successChan)

		clients.Execute(func(kube client.Kube) error {
			items, err := lister(context.Background(), kube, getCommonNamespace(kube, namespace), metav1.ListOptions{})
			if err != nil {
				return err
			}

			for _, deployment := range items {
				output <- deployment.GetName()
			}

			successChan <- true

			return nil
		})
	}()

	var items []string
	for item := range output {
		items = append(items, item)
	}

	var successCount uint64
	for range successChan {
		successCount++
	}

	return uniqueAndPresent(items, successCount)
}

func uniqueAndPresent(items []string, wantedCount uint64) []string {
	sort.Strings(items)

	var count uint64
	var previous string

	unique := items[:0]

	for _, item := range items {
		if item == previous {
			count++
		} else {
			count = 1
			previous = item
		}

		if count == wantedCount {
			unique = append(unique, previous)
		}
	}

	return unique
}
