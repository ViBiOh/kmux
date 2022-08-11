package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

var Resources = []string{
	"cronjobs",
	"daemonsets",
	"deployments",
	"jobs",
	"namespaces",
	"services",
}

type PodWatcher func(context.Context, client.Kube) (watch.Interface, error)

func GetPodWatcher(resourceType, resourceName string) PodWatcher {
	return func(ctx context.Context, kube client.Kube) (watch.Interface, error) {
		var labelGetter func(context.Context, client.Kube, string) (string, error)

		switch resourceType {
		case "cj", "cronjob", "cronjobs":
			labelGetter = getCronJobLabelSelector
		case "ds", "daemonset", "daemonsets":
			labelGetter = getDaemonSetLabelSelector
		case "deploy", "deployment", "deployments":
			labelGetter = getDeploymentLabelSelector
		case "job", "jobs":
			labelGetter = getJobLabelSelector
		case "ns", "namespace", "namespaces":
			return kube.CoreV1().Pods(resourceName).Watch(ctx, metav1.ListOptions{
				Watch: true,
			})
		case "sts", "statefulset", "statefulsets":
			labelGetter = getStatefulSetSelector
		case "svc", "service", "services":
			labelGetter = getServiceLabelSelector
		default:
			return nil, fmt.Errorf("unhandled resource type `%s` for log", resourceType)
		}

		labelSelector, err := labelGetter(ctx, kube, resourceName)
		if err != nil {
			return nil, err
		}

		return kube.CoreV1().Pods(kube.Namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Watch:         true,
		})
	}
}

func getCronJobLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	cronjob, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fromLabelSelector(cronjob.Spec.JobTemplate.Spec.Selector), nil
}

func getDaemonSetLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	daemonSet, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fromLabelSelector(daemonSet.Spec.Selector), nil
}

func getDeploymentLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	deployment, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fromLabelSelector(deployment.Spec.Selector), nil
}

func getJobLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	job, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fromLabelSelector(job.Spec.Selector), nil
}

func getStatefulSetSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	statefulSet, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fromLabelSelector(statefulSet.Spec.Selector), nil
}

func getServiceLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	service, err := kube.CoreV1().Services(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fromLabels(service.Spec.Selector), nil
}

func fromLabelSelector(selector *metav1.LabelSelector) string {
	return fromLabels(selector.MatchLabels)
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
