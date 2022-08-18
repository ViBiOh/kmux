package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type (
	PodsWatcher func(context.Context, client.Kube) (watch.Interface, error)
	PodsGetter  func(context.Context, client.Kube) (*v1.PodList, error)
)

func GetPodsWatcher(resourceType, resourceName string) PodsWatcher {
	return func(ctx context.Context, kube client.Kube) (watch.Interface, error) {
		namespace, listOptions, err := PodsGetterConfiguration(ctx, kube, resourceType, resourceName)
		if err != nil {
			return nil, err
		}

		listOptions.Watch = true

		return kube.CoreV1().Pods(namespace).Watch(ctx, listOptions)
	}
}

func GetPodsGetter(resourceType, resourceName string) PodsGetter {
	return func(ctx context.Context, kube client.Kube) (*v1.PodList, error) {
		namespace, listOptions, err := PodsGetterConfiguration(ctx, kube, resourceType, resourceName)
		if err != nil {
			return nil, err
		}

		return kube.CoreV1().Pods(namespace).List(ctx, listOptions)
	}
}

func PodsGetterConfiguration(ctx context.Context, kube client.Kube, resourceType, resourceName string) (namespace string, options metav1.ListOptions, err error) {
	var matchLabels map[string]string

	switch resourceType {
	case "ns", "namespace", "namespaces":
		namespace = resourceName
		return

	case "po", "pod", "pods",
		"no", "node", "nodes":
		namespace = kube.Namespace
		options.FieldSelector, err = PodFieldSelectorGetter(ctx, resourceType, resourceName)
		return

	case "svc", "service", "services":
		var service *v1.Service
		service, err = kube.CoreV1().Services(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return
		}

		matchLabels = service.Spec.Selector

	default:
		var labelSelector *metav1.LabelSelector
		labelSelector, err = PodLabelSelectorGetter(ctx, kube, resourceType, resourceName)
		if err != nil {
			return
		}

		if labelSelector != nil {
			matchLabels = labelSelector.MatchLabels
		}
	}

	namespace = kube.Namespace
	options.LabelSelector = fromMaps(matchLabels)

	return
}

func fromMaps(labels map[string]string) string {
	var labelSelector strings.Builder

	for key, value := range labels {
		if labelSelector.Len() > 0 {
			labelSelector.WriteString(",")
		}

		labelSelector.WriteString(fmt.Sprintf("%s=%s", key, value))
	}

	return labelSelector.String()
}
