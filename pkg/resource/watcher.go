package resource

import (
	"context"
	"fmt"
	"runtime"

	"github.com/ViBiOh/kmux/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type WrappedWatcher struct {
	stop func()
	pods chan watch.Event
}

func (dw WrappedWatcher) Stop() {
	if dw.stop != nil {
		dw.stop()
	}
}

func (dw WrappedWatcher) ResultChan() <-chan watch.Event {
	return dw.pods
}

func WatchPods(ctx context.Context, kube client.Kube, kind, name string, labelSelector map[string]string, dryRun bool) (watch.Interface, error) {
	var listOptions metav1.ListOptions
	var postListFilter PodFilter
	var err error

	namespace := kube.Namespace

	if len(kind) > 0 && len(name) > 0 {
		namespace, listOptions, postListFilter, err = GetPodsSelector(ctx, kube, kind, name)
		if err != nil {
			return nil, fmt.Errorf("get list options: %w", err)
		}
	}

	if len(labelSelector) > 0 {
		labelSelector := labelSelectorFromMaps(labelSelector)
		if len(listOptions.LabelSelector) > 0 {
			listOptions.LabelSelector += ","
		}

		listOptions.LabelSelector += labelSelector
	}

	if dryRun {
		return watchPodsDry(ctx, kube, namespace, listOptions, postListFilter)
	}

	listOptions.Watch = true

	return watchPods(ctx, kube, namespace, listOptions, postListFilter)
}

func watchPods(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions, postListFilter PodFilter) (watch.Interface, error) {
	watcher, err := kube.CoreV1().Pods(namespace).Watch(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("watch: %w", err)
	}

	if postListFilter == nil {
		return watcher, nil
	}

	podsChan := make(chan watch.Event, runtime.NumCPU())

	go func() {
		defer close(podsChan)

		for event := range watcher.ResultChan() {
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				continue
			}

			if postListFilter(ctx, kube, *pod) {
				podsChan <- event
			}
		}
	}()

	return WrappedWatcher{
		stop: watcher.Stop,
		pods: podsChan,
	}, nil
}

func watchPodsDry(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions, postListFilter PodFilter) (watch.Interface, error) {
	pods, err := kube.CoreV1().Pods(namespace).List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	items := pods.Items[:0]
	for _, pod := range pods.Items {
		if postListFilter == nil || postListFilter(ctx, kube, pod) {
			items = append(items, pod)
		}
	}

	podsChan := make(chan watch.Event, len(items))

	go func() {
		defer close(podsChan)

		for _, pod := range items {
			pod := pod
			podsChan <- watch.Event{
				Type:   watch.Added,
				Object: &pod,
			}
		}
	}()

	return WrappedWatcher{
		pods: podsChan,
	}, nil
}
