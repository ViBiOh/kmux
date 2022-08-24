package resource

import (
	"context"

	"github.com/ViBiOh/kmux/pkg/client"
	"k8s.io/apimachinery/pkg/watch"
)

type DryWatcher struct {
	pods chan watch.Event
}

func (dw DryWatcher) Stop() {
	// Nothing to stop
}

func (dw DryWatcher) ResultChan() <-chan watch.Event {
	return dw.pods
}

func WatchPods(ctx context.Context, kube client.Kube, resourceType, resourceName string, dryRun bool) (watch.Interface, error) {
	if dryRun {
		return watchPodsDry(ctx, kube, resourceType, resourceName)
	}

	namespace, listOptions, err := PodsGetterConfiguration(ctx, kube, resourceType, resourceName)
	if err != nil {
		return nil, err
	}

	listOptions.Watch = true

	return kube.CoreV1().Pods(namespace).Watch(ctx, listOptions)
}

func watchPodsDry(ctx context.Context, kube client.Kube, resourceType, resourceName string) (watch.Interface, error) {
	pods, err := ListPods(ctx, kube, resourceType, resourceName)
	if err != nil {
		return nil, err
	}

	podsChan := make(chan watch.Event, len(pods.Items))

	go func() {
		defer close(podsChan)

		for _, pod := range pods.Items {
			pod := pod
			podsChan <- watch.Event{
				Type:   watch.Added,
				Object: &pod,
			}
		}
	}()

	return DryWatcher{
		pods: podsChan,
	}, nil
}
