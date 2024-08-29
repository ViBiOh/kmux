package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodFilter func(context.Context, client.Kube, v1.Pod) bool

func GetPodSpec(ctx context.Context, kube client.Kube, resourceType, resourceName string) (v1.PodSpec, error) {
	switch resourceType {
	case "cj", "cronjob", "cronjobs":
		item, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.JobTemplate.Spec.Template.Spec, nil

	case "ds", "daemonset", "daemonsets":
		item, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "deploy", "deployment", "deployments":
		item, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "job", "jobs":
		item, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "po", "pod", "pods":
		item, err := kube.CoreV1().Pods(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec, nil

	case "rs", "replicaset", "replicasets":
		item, err := kube.AppsV1().ReplicaSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "sts", "statefulset", "statefulsets":
		item, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	default:
		return v1.PodSpec{}, unhandledError(resourceType)
	}
}

func GetPodsSelector(ctx context.Context, kube client.Kube, resourceType, resourceName string) (namespace string, options metav1.ListOptions, postListFilter PodFilter, err error) {
	switch resourceType {
	case "ns", "namespace", "namespaces":
		if len(resourceName) == 0 {
			namespace = kube.Namespace
		} else {
			namespace = resourceName
		}

		return

	case "po", "pod", "pods",
		"no", "node", "nodes":
		namespace = kube.Namespace
		options.FieldSelector, err = podFieldSelectorGetter(resourceType, resourceName)

		return

	case "svc", "service", "services":
		var service *v1.Service
		service, err = kube.CoreV1().Services(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("get services: %w", err)

			return
		}

		namespace = kube.Namespace
		options.LabelSelector = labelSelectorFromMaps(service.Spec.Selector)

		return

	case "cj", "cronjob", "cronjobs":
		var cronjob *batchv1.CronJob
		cronjob, err = kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("get cronjob: %w", err)

			return
		}

		namespace = kube.Namespace
		options.LabelSelector = "job-name"
		postListFilter = func(ctx context.Context, kube client.Kube, pod v1.Pod) bool {
			for _, podReference := range pod.ObjectMeta.OwnerReferences {
				if podReference.Kind != "Job" {
					continue
				}

				job, err := kube.BatchV1().Jobs(cronjob.Namespace).Get(ctx, podReference.Name, metav1.GetOptions{})
				if err != nil {
					kube.Warn("get job `%s`: %s", podReference.Name, err)

					continue
				}

				for _, jobReference := range job.ObjectMeta.OwnerReferences {
					if jobReference.UID == cronjob.UID {
						return true
					}
				}
			}

			return false
		}

		return

	default:
		var labelSelector *metav1.LabelSelector
		labelSelector, err = podLabelSelectorGetter(ctx, kube, resourceType, resourceName)
		if err != nil {
			return
		}

		namespace = kube.Namespace
		options.LabelSelector = labelSelectorFromMaps(labelSelector.MatchLabels)

		return
	}
}

func podLabelSelectorGetter(ctx context.Context, kube client.Kube, resourceType, resourceName string) (*metav1.LabelSelector, error) {
	switch resourceType {
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

func podFieldSelectorGetter(resourceType, resourceName string) (string, error) {
	switch resourceType {
	case "po", "pod", "pods":
		return fmt.Sprintf("metadata.name=%s", resourceName), nil

	case "no", "node", "nodes":
		return fmt.Sprintf("spec.nodeName=%s", resourceName), nil

	default:
		return "", unhandledError(resourceType)
	}
}

func labelSelectorFromMaps(labels map[string]string) string {
	var labelSelector strings.Builder

	for key, value := range labels {
		if labelSelector.Len() > 0 {
			labelSelector.WriteString(",")
		}

		labelSelector.WriteString(fmt.Sprintf("%s=%s", key, value))
	}

	return labelSelector.String()
}

func unhandledError(resourceType string) error {
	return fmt.Errorf("unhandled resource type `%s`", resourceType)
}
