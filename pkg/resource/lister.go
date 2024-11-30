package resource

import (
	"context"
	"fmt"

	"github.com/ViBiOh/kmux/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Lister func(context.Context, client.Kube, string) ([]string, error)

func ListerFor(kind string) (Lister, error) {
	switch kind {
	case "cj", "cronjob", "cronjobs":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "ds", "daemonset", "daemonsets":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "deploy", "deployment", "deployments":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "job", "jobs":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "po", "pod", "pods":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "ns", "namespace", "namespaces":
		return func(ctx context.Context, kube client.Kube, _ string) ([]string, error) {
			items, err := kube.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "no", "node", "nodes":
		return func(ctx context.Context, kube client.Kube, _ string) ([]string, error) {
			items, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "svc", "service", "services":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	case "sts", "statefulset", "statefulsets":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			items, err := kube.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}

			output := make([]string, len(items.Items))
			for i, item := range items.Items {
				output[i] = item.ObjectMeta.GetName()
			}

			return output, nil
		}, nil

	default:
		return nil, fmt.Errorf("unhandled resource type `%s`", kind)
	}
}

func ListPods(ctx context.Context, kube client.Kube, kind, name string) ([]v1.Pod, error) {
	namespace, options, filter, err := GetPodsSelector(ctx, kube, kind, name)
	if err != nil {
		return nil, fmt.Errorf("get pods selector: %w", err)
	}

	pods, err := kube.CoreV1().Pods(namespace).List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("get pods: %w", err)
	}

	if filter == nil {
		return pods.Items, nil
	}

	var output []v1.Pod
	for _, pod := range pods.Items {
		if filter(ctx, kube, pod) {
			output = append(output, pod)
		}
	}

	return output, nil
}
