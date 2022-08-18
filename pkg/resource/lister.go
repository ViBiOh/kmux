package resource

import (
	"context"
	"fmt"

	"github.com/ViBiOh/kmux/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Lister func(context.Context, client.Kube, string, metav1.ListOptions) ([]metav1.ObjectMeta, error)

func ListerFor(resourceType string) (Lister, error) {
	switch resourceType {
	case "cj", "cronjob", "cronjobs":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.BatchV1().CronJobs(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "ds", "daemonset", "daemonsets":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.AppsV1().DaemonSets(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "deploy", "deployment", "deployments":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.AppsV1().Deployments(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "job", "jobs":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.BatchV1().Jobs(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "po", "pod", "pods":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.CoreV1().Pods(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "ns", "namespace", "namespaces":
		return func(ctx context.Context, kube client.Kube, _ string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.CoreV1().Namespaces().List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "no", "node", "nodes":
		return func(ctx context.Context, kube client.Kube, _ string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.CoreV1().Nodes().List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "svc", "service", "services":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.CoreV1().Services(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	case "sts", "statefulset", "statefulsets":
		return func(ctx context.Context, kube client.Kube, namespace string, options metav1.ListOptions) ([]metav1.ObjectMeta, error) {
			items, err := kube.AppsV1().StatefulSets(namespace).List(ctx, options)
			if err != nil {
				return nil, err
			}

			output := make([]metav1.ObjectMeta, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta
			}

			return output, nil
		}, nil
	default:
		return nil, fmt.Errorf("unhandled resource type `%s`", resourceType)
	}
}
