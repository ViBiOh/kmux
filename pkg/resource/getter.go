package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodTemplateGetter(ctx context.Context, kube client.Kube, resourceType, resourceName string) (v1.PodTemplateSpec, error) {
	switch resourceType {
	case "cronjob", "cronjobs":
		item, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}

		return item.Spec.JobTemplate.Spec.Template, nil
	case "ds", "daemonset", "daemonsets":
		item, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}

		return item.Spec.Template, nil
	case "deploy", "deployment", "deployments":
		item, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}

		return item.Spec.Template, nil
	case "job", "jobs":
		item, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}

		return item.Spec.Template, nil
	case "sts", "statefulset", "statefulsets":
		item, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}

		return item.Spec.Template, nil
	default:
		return v1.PodTemplateSpec{}, unhandledError(resourceType)
	}
}

func ListPods(ctx context.Context, kube client.Kube, resourceType, resourceName string) (*v1.PodList, error) {
	namespace, listOptions, err := PodsGetterConfiguration(ctx, kube, resourceType, resourceName)
	if err != nil {
		return nil, err
	}

	return kube.CoreV1().Pods(namespace).List(ctx, listOptions)
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

func PodLabelSelectorGetter(ctx context.Context, kube client.Kube, resourceType, resourceName string) (*metav1.LabelSelector, error) {
	switch resourceType {
	case "cronjob", "cronjobs":
		item, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.JobTemplate.Spec.Selector, nil
	case "ds", "daemonset", "daemonsets":
		item, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil
	case "deploy", "deployment", "deployments":
		item, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil
	case "job", "jobs":
		item, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil
	case "sts", "statefulset", "statefulsets":
		item, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil
	default:
		return nil, unhandledError(resourceType)
	}
}

func PodFieldSelectorGetter(ctx context.Context, resourceType, resourceName string) (string, error) {
	switch resourceType {
	case "po", "pod", "pods":
		return fmt.Sprintf("metadata.name=%s", resourceName), nil
	case "no", "node", "nodes":
		return fmt.Sprintf("spec.nodeName=%s", resourceName), nil
	default:
		return "", unhandledError(resourceType)
	}
}

func unhandledError(resourceType string) error {
	return fmt.Errorf("unhandled resource type `%s`", resourceType)
}
