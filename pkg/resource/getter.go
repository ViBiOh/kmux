package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodFilter func(context.Context, client.Kube, v1.Pod) bool

type Replicable interface {
	GetReplicas() *int32
}

type ReplicableDeployment struct {
	appsv1.DeploymentSpec
}

func (rd ReplicableDeployment) GetReplicas() *int32 {
	return rd.Replicas
}

type ReplicableReplicaSet struct {
	appsv1.ReplicaSetSpec
}

func (rd ReplicableReplicaSet) GetReplicas() *int32 {
	return rd.Replicas
}

type ReplicableStatefulSet struct {
	appsv1.StatefulSetSpec
}

func (rd ReplicableStatefulSet) GetReplicas() *int32 {
	return rd.Replicas
}

func GetScale(ctx context.Context, kube client.Kube, kind, name string) (*autoscalingv1.Scale, error) {
	switch kind {
	case "deploy", "deployment", "deployments":
		return kube.AppsV1().Deployments(kube.Namespace).GetScale(ctx, name, metav1.GetOptions{})

	case "rs", "replicaset", "replicasets":
		return kube.AppsV1().ReplicaSets(kube.Namespace).GetScale(ctx, name, metav1.GetOptions{})

	case "sts", "statefulset", "statefulsets":
		return kube.AppsV1().StatefulSets(kube.Namespace).GetScale(ctx, name, metav1.GetOptions{})

	default:
		return nil, unhandledError(kind)
	}
}

func GetPodSpec(ctx context.Context, kube client.Kube, kind, name string) (v1.PodSpec, error) {
	switch kind {
	case "cj", "cronjob", "cronjobs":
		item, err := kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.JobTemplate.Spec.Template.Spec, nil

	case "ds", "daemonset", "daemonsets":
		item, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "deploy", "deployment", "deployments":
		item, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "job", "jobs":
		item, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "po", "pod", "pods":
		item, err := kube.CoreV1().Pods(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec, nil

	case "rs", "replicaset", "replicasets":
		item, err := kube.AppsV1().ReplicaSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	case "sts", "statefulset", "statefulsets":
		item, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return v1.PodSpec{}, err
		}

		return item.Spec.Template.Spec, nil

	default:
		return v1.PodSpec{}, unhandledError(kind)
	}
}

func GetPodsSelector(ctx context.Context, kube client.Kube, kind, name string) (namespace string, options metav1.ListOptions, postListFilter PodFilter, err error) {
	switch kind {
	case "ns", "namespace", "namespaces":
		if len(name) == 0 {
			namespace = kube.Namespace
		} else {
			namespace = name
		}

		return namespace, options, postListFilter, err

	case "po", "pod", "pods",
		"no", "node", "nodes":
		namespace = kube.Namespace
		options.FieldSelector, err = podFieldSelectorGetter(kind, name)

		return namespace, options, postListFilter, err

	case "svc", "service", "services":
		var service *v1.Service
		service, err = kube.CoreV1().Services(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("get services: %w", err)

			return namespace, options, postListFilter, err
		}

		namespace = kube.Namespace
		options.LabelSelector = labelSelectorFromMaps(service.Spec.Selector)

		return namespace, options, postListFilter, err

	case "cj", "cronjob", "cronjobs":
		var cronjob *batchv1.CronJob
		cronjob, err = kube.BatchV1().CronJobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return namespace, options, postListFilter, fmt.Errorf("get cronjob: %w", err)
		}

		namespace = kube.Namespace
		options.LabelSelector = "job-name"
		postListFilter = func(ctx context.Context, kube client.Kube, pod v1.Pod) bool {
			for _, podReference := range pod.OwnerReferences {
				if podReference.Kind != "Job" {
					continue
				}

				job, err := kube.BatchV1().Jobs(cronjob.Namespace).Get(ctx, podReference.Name, metav1.GetOptions{})
				if err != nil {
					kube.Warn("get job `%s`: %s", podReference.Name, err)

					continue
				}

				for _, jobReference := range job.OwnerReferences {
					if jobReference.UID == cronjob.UID {
						return true
					}
				}
			}

			return false
		}

		return namespace, options, postListFilter, err

	default:
		var labelSelector *metav1.LabelSelector
		labelSelector, err = podLabelSelectorGetter(ctx, kube, kind, name)
		if err != nil {
			return namespace, options, postListFilter, err
		}

		namespace = kube.Namespace
		options.LabelSelector = labelSelectorFromMaps(labelSelector.MatchLabels)

		return namespace, options, postListFilter, err
	}
}

func podLabelSelectorGetter(ctx context.Context, kube client.Kube, kind, name string) (*metav1.LabelSelector, error) {
	switch kind {
	case "ds", "daemonset", "daemonsets":
		item, err := kube.AppsV1().DaemonSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil

	case "deploy", "deployment", "deployments":
		item, err := kube.AppsV1().Deployments(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil

	case "job", "jobs":
		item, err := kube.BatchV1().Jobs(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil

	case "sts", "statefulset", "statefulsets":
		item, err := kube.AppsV1().StatefulSets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return item.Spec.Selector, nil

	default:
		return nil, unhandledError(kind)
	}
}

func podFieldSelectorGetter(kind, name string) (string, error) {
	switch kind {
	case "po", "pod", "pods":
		return fmt.Sprintf("metadata.name=%s", name), nil

	case "no", "node", "nodes":
		return fmt.Sprintf("spec.nodeName=%s", name), nil

	default:
		return "", unhandledError(kind)
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

func unhandledError(kind string) error {
	return fmt.Errorf("unhandled resource type `%s`", kind)
}
