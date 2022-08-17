package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type PodWatcher func(context.Context, client.Kube) (watch.Interface, error)

func GetPodWatcher(resourceType, resourceName string) PodWatcher {
	return func(ctx context.Context, kube client.Kube) (watch.Interface, error) {
		var matchLabels map[string]string

		switch resourceType {
		case "ns", "namespace", "namespaces":
			return kube.CoreV1().Pods(resourceName).Watch(ctx, metav1.ListOptions{
				Watch: true,
			})

		case "no", "node", "nodes":
			return kube.CoreV1().Pods(kube.Namespace).Watch(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("spec.nodeName=%s", resourceName),
				Watch:         true,
			})

		case "svc", "service", "services":
			service, err := kube.CoreV1().Services(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			matchLabels = service.Spec.Selector

		default:
			labelSelector, err := PodSelectorGetter(ctx, kube, resourceType, resourceName)
			if err != nil {
				return nil, err
			}

			if labelSelector != nil {
				matchLabels = labelSelector.MatchLabels
			}
		}

		return kube.CoreV1().Pods(kube.Namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: fromLabels(matchLabels),
			Watch:         true,
		})
	}
}

func fromLabels(labels map[string]string) string {
	var labelSelector strings.Builder

	for key, value := range labels {
		if labelSelector.Len() > 0 {
			labelSelector.WriteString(",")
		}

		labelSelector.WriteString(fmt.Sprintf("%s=%s", key, value))
	}

	return labelSelector.String()
}
