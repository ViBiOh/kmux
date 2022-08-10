package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kube/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

var Resources = []string{
	"deployments",
	"cronjobs",
	"jobs",
	"daemonsets",
}

var ResourcesAliases = []string{
	"deploy",
	"deployment",
	"cj",
	"cronjob",
	"job",
	"ds",
	"daemonset",
}

type PodWatcher func(context.Context, client.Kube) (watch.Interface, error)

func WatcherLabelSelector(resourceType, resourceName string) PodWatcher {
	return func(ctx context.Context, kube client.Kube) (watch.Interface, error) {
		var labelGetter func(context.Context, client.Kube, string) (string, error)

		switch resourceType {
		case "deploy", "deployment", "deployments":
			labelGetter = getDeploymentLabelSelector
		case "cj", "cronjob", "cronjobs":
			labelGetter = getCronJobLabelSelector
		case "job", "jobs":
			labelGetter = getJobLabelSelector
		case "ds", "daemonset", "daemonsets":
			labelGetter = getDaemonSetLabelSelector
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

func getDeploymentLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	deployment, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(deployment.Spec.Selector), nil
}

func getDaemonSetLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	deployment, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(deployment.Spec.Selector), nil
}

func getCronJobLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	job, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(job.Spec.JobTemplate.Spec.Selector), nil
}

func getJobLabelSelector(ctx context.Context, kube client.Kube, name string) (string, error) {
	job, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(job.Spec.Selector), nil
}

func toLabelSelector(selector *metav1.LabelSelector) string {
	var labelSelector strings.Builder

	for key, value := range selector.MatchLabels {
		if labelSelector.Len() > 0 {
			labelSelector.WriteString(",")
		}
		labelSelector.WriteString(fmt.Sprintf("%s=%s", key, value))
	}

	return labelSelector.String()
}
