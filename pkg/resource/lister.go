package resource

import (
	"context"
	"fmt"

	"github.com/ViBiOh/kmux/pkg/client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Lister func(context.Context, client.Kube, string) ([]string, error)

func ListerFor(kind string) (Lister, error) {
	switch kind {
	case "cj", "cronjob", "cronjobs":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "ds", "daemonset", "daemonsets":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "deploy", "deployment", "deployments":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "job", "jobs":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "po", "pod", "pods":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "rs", "replicaset", "replicasets":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "sts", "statefulset", "statefulsets":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "svc", "service", "services":
		return func(ctx context.Context, kube client.Kube, namespace string) ([]string, error) {
			return itemsName(kube.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{}))
		}, nil

	case "ns", "namespace", "namespaces":
		return func(ctx context.Context, kube client.Kube, _ string) ([]string, error) {
			return itemsName(kube.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}))
		}, nil

	case "no", "node", "nodes":
		return func(ctx context.Context, kube client.Kube, _ string) ([]string, error) {
			return itemsName(kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{}))
		}, nil

	default:
		return nil, unhandledError(kind)
	}
}

// itemsName extracts the name of every listed object. The list error is
// forwarded so every kind stays a one liner.
func itemsName(list runtime.Object, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, fmt.Errorf("extract list: %w", err)
	}

	output := make([]string, 0, len(items))

	for _, item := range items {
		object, ok := item.(metav1.Object)
		if !ok {
			continue
		}

		output = append(output, object.GetName())
	}

	return output, nil
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
