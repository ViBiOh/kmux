package cmd

import (
	"context"
	"sort"
	"sync"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
)

func getNamespace(kube client.Kube, namespace string) string {
	if len(namespace) != 0 {
		return namespace
	}

	if !allNamespace {
		return kube.Namespace
	}

	return ""
}

func listCommonObjects(ctx context.Context, namespace string, lister resource.Lister) []string {
	var mutex sync.Mutex

	counts := make(map[string]uint64)

	var successCount uint64

	clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
		items, err := lister(ctx, kube, getNamespace(kube, namespace))
		if err != nil {
			return err
		}

		seen := make(map[string]struct{}, len(items))

		mutex.Lock()
		defer mutex.Unlock()

		successCount++

		for _, item := range items {
			if _, ok := seen[item]; ok {
				continue
			}

			seen[item] = struct{}{}
			counts[item]++
		}

		return nil
	})

	items := make([]string, 0, len(counts))

	for item, count := range counts {
		if count == successCount {
			items = append(items, item)
		}
	}

	sort.Strings(items)

	return items
}
