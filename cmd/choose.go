package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/ViBiOh/kube/pkg/client"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nolint deadcode
func getAllDeployments(name string) []string {
	output := make(chan string, len(clients))
	successChan := make(chan bool, len(clients))

	go func() {
		defer close(output)
		defer close(successChan)

		clients.Execute(func(kube client.Kube) error {
			list, err := kube.AppsV1().Deployments("").List(context.Background(), v1.ListOptions{})
			if err != nil {
				return err
			}

			for _, deployment := range list.Items {
				output <- fmt.Sprintf("%s/%s", deployment.GetNamespace(), deployment.GetName())
			}

			successChan <- true

			return nil
		})
	}()

	var items []string
	for item := range output {
		items = append(items, item)
	}
	sort.Strings(items)

	var successCount uint64
	for range successChan {
		successCount += 1
	}

	deduplicated := items[:0]
	var count uint64
	var previous string

	for _, item := range items {
		if item == previous {
			count += 1
		} else {
			count = 1
			previous = item
		}

		if count == successCount {
			deduplicated = append(deduplicated, previous)
		}
	}

	return deduplicated
}
