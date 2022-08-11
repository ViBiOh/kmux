package resource

import (
	"context"
	"fmt"

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
		return v1.PodTemplateSpec{}, fmt.Errorf("unhandled resource type `%s`", resourceType)
	}
}

func PodSelectorGetter(ctx context.Context, kube client.Kube, resourceType, resourceName string) (*metav1.LabelSelector, error) {
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
		return nil, fmt.Errorf("unhandled resource type `%s`", resourceType)
	}
}
