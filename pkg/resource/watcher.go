package resource

import (
	"context"

	"github.com/ViBiOh/kmux/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func WatchPods(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions, dryRun bool) (watch.Interface, error) {
	if dryRun {
		return watchPodsDry(ctx, kube, namespace, options)
	}

	options.Watch = true

	return kube.CoreV1().Pods(namespace).Watch(ctx, options)
}

func watchPodsDry(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) (watch.Interface, error) {
	pods, err := kube.CoreV1().Pods(namespace).List(ctx, options)
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
