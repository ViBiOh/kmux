package env

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	v1 "k8s.io/api/core/v1"
	kubeResource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	envLabels      = regexp.MustCompile(`(?m)metadata\.labels\[["']?(.*?)["']?\]`)
	envAnnotations = regexp.MustCompile(`(?m)metadata\.annotations\[["']?(.*?)["']?\]`)
)

type envValue struct {
	data   map[string]string
	source string
}

func (ev envValue) String() string {
	output := output.Yellow.Sprintf("# %s\n", ev.source)

	entries := make([]string, 0, len(ev.data))

	for key, value := range ev.data {
		entries = append(entries, fmt.Sprintf("%s=%s\n", key, value))
	}

	sort.Strings(entries)

	return output + strings.Join(entries, "")
}

type EnvGetter struct {
	containerRegexp *regexp.Regexp
	resourceType    string
	resourceName    string
}

func NewEnvGetter(resourceType, resourceName string) EnvGetter {
	return EnvGetter{
		resourceType: resourceType,
		resourceName: resourceName,
	}
}

func (eg EnvGetter) WithContainerRegexp(containerRegexp *regexp.Regexp) EnvGetter {
	eg.containerRegexp = containerRegexp

	return eg
}

func (eg EnvGetter) Get(ctx context.Context, kube client.Kube) error {
	podSpec, err := resource.GetPodSpec(ctx, kube, eg.resourceType, eg.resourceName)
	if err != nil {
		return err
	}

	pods, err := resource.ListPods(ctx, kube, eg.resourceType, eg.resourceName)
	if err != nil {
		return err
	}

	mostLivePod := getMostLivePod(pods)

	var node v1.Node

	if len(mostLivePod.Spec.NodeName) != 0 {
		podNode, err := kube.CoreV1().Nodes().Get(ctx, mostLivePod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		node = *podNode
	}

	var containers []v1.Container

	for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
		if !resource.IsContainedSelected(container, eg.containerRegexp) {
			continue
		}

		containers = append(containers, container)
	}

	for _, container := range containers {
		values := getEnv(ctx, kube, container, mostLivePod, node)

		if len(values) == 0 {
			continue
		}

		containerOutput := &strings.Builder{}

		for _, value := range values {
			fmt.Fprintf(containerOutput, "%s", value)
		}

		outputter := kube.Outputter

		if len(containers) != 1 {
			outputter = kube.Outputter.Child(false, output.Green.Sprintf("[%s]", container.Name))
		}

		outputter.Info("%s", containerOutput.String())
	}

	return nil
}

func getMostLivePod(pods []v1.Pod) v1.Pod {
	for _, pod := range pods {
		if pod.Status.Phase == v1.PodRunning {
			return pod
		}
	}

	for _, pod := range pods {
		if pod.Status.Phase == v1.PodSucceeded {
			return pod
		}
	}

	for _, pod := range pods {
		if pod.Status.Phase == v1.PodFailed {
			return pod
		}
	}

	for _, pod := range pods {
		if pod.Status.Phase == v1.PodPending {
			return pod
		}
	}

	for _, pod := range pods {
		if pod.Status.Phase == v1.PodUnknown {
			return pod
		}
	}

	return v1.Pod{}
}

func getEnv(ctx context.Context, kube client.Kube, container v1.Container, pod v1.Pod, node v1.Node) []envValue {
	var output []envValue

	configMaps, secrets := getEnvDependencies(ctx, kube, container)

	for _, envFrom := range container.EnvFrom {
		var key string
		var content map[string]string

		if envFrom.ConfigMapRef != nil {
			key, content = getEnvFromSource(configMaps, "configmap", envFrom.Prefix, envFrom.ConfigMapRef.Name, envFrom.ConfigMapRef.Optional)
		} else if envFrom.SecretRef != nil {
			key, content = getEnvFromSource(secrets, "secret", envFrom.Prefix, envFrom.SecretRef.Name, envFrom.SecretRef.Optional)
		}

		output = append(output, envValue{
			source: key,
			data:   content,
		})
	}

	if len(container.Env) > 0 {
		inline := make(map[string]string)

		for _, env := range container.Env {
			inline[env.Name] = getInlineEnv(pod, node, env, configMaps, secrets)
		}

		output = append(output, envValue{
			source: "inline",
			data:   inline,
		})
	}

	return output
}

func getEnvDependencies(ctx context.Context, kube client.Kube, container v1.Container) (map[string]map[string]string, map[string]map[string]string) {
	configMaps, secrets := gatherEnvDependencies(container)

	for name := range configMaps {
		configMap, err := kube.CoreV1().ConfigMaps(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			kube.Err("getting configmap `%s`: %s", name, err)
			continue
		}

		configMaps[name] = configMap.Data
	}

	for name := range secrets {
		secret, err := kube.CoreV1().Secrets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			kube.Err("getting secret `%s`: %s", name, err)
			continue
		}

		secrets[name] = make(map[string]string)

		for key, value := range secret.Data {
			secrets[name][key] = string(value)
		}
	}

	return configMaps, secrets
}

func gatherEnvDependencies(container v1.Container) (map[string]map[string]string, map[string]map[string]string) {
	configMaps := make(map[string]map[string]string)
	secrets := make(map[string]map[string]string)

	for _, env := range container.Env {
		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				configMaps[env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name] = nil
			} else if env.ValueFrom.SecretKeyRef != nil {
				secrets[env.ValueFrom.SecretKeyRef.LocalObjectReference.Name] = nil
			}
		}
	}

	for _, envFrom := range container.EnvFrom {
		if envFrom.ConfigMapRef != nil {
			configMaps[envFrom.ConfigMapRef.LocalObjectReference.Name] = nil
		} else if envFrom.SecretRef != nil {
			secrets[envFrom.SecretRef.LocalObjectReference.Name] = nil
		}
	}

	return configMaps, secrets
}

func getEnvFromSource(storage map[string]map[string]string, kind, prefix, name string, optional *bool) (string, map[string]string) {
	content := make(map[string]string)
	keyName := fmt.Sprintf("%s %s", kind, name)

	values, ok := storage[name]
	if !ok {
		if optional != nil && !*optional {
			content["error"] = fmt.Sprintf("<%s not optional and not found>", kind)

			return keyName, content
		}
	}

	for key, value := range values {
		content[prefix+key] = value
	}

	return keyName, content
}

func getInlineEnv(pod v1.Pod, node v1.Node, envVar v1.EnvVar, configMaps, secrets map[string]map[string]string) string {
	if len(envVar.Value) > 0 {
		return envVar.Value
	}

	return getValueFrom(pod, node, envVar, configMaps, secrets)
}

func getValueFrom(pod v1.Pod, node v1.Node, envVar v1.EnvVar, configMaps, secrets map[string]map[string]string) string {
	if envVar.ValueFrom.ConfigMapKeyRef != nil {
		return getValueFromRef(configMaps, "configmap", envVar.ValueFrom.ConfigMapKeyRef.Name, envVar.ValueFrom.ConfigMapKeyRef.Key, envVar.ValueFrom.ConfigMapKeyRef.Optional)
	}

	if envVar.ValueFrom.SecretKeyRef != nil {
		return getValueFromRef(secrets, "secret", envVar.ValueFrom.SecretKeyRef.Name, envVar.ValueFrom.SecretKeyRef.Key, envVar.ValueFrom.SecretKeyRef.Optional)
	}

	if envVar.ValueFrom.FieldRef != nil {
		return getEnvFieldRef(pod, *envVar.ValueFrom.FieldRef)
	}

	if envVar.ValueFrom.ResourceFieldRef != nil {
		return getEnvResourceRef(pod.Spec, node, *envVar.ValueFrom.ResourceFieldRef)
	}

	return ""
}

func getValueFromRef(storage map[string]map[string]string, kind, name, key string, optional *bool) string {
	values, ok := storage[name]
	if !ok {
		if optional != nil && !*optional {
			return fmt.Sprintf("<%s `%s` not optional and not found>", kind, name)
		}
	}

	return values[key]
}

func getEnvFieldRef(pod v1.Pod, field v1.ObjectFieldSelector) string {
	if matches := envLabels.FindAllStringSubmatch(field.FieldPath, -1); len(matches) > 0 {
		return pod.Labels[matches[0][1]]
	}

	if matches := envAnnotations.FindAllStringSubmatch(field.FieldPath, -1); len(matches) > 0 {
		return pod.Annotations[matches[0][1]]
	}

	switch field.FieldPath {
	case "metadata.name":
		return pod.GetName()

	case "metadata.namespace":
		return pod.GetNamespace()

	case "spec.nodeName":
		return pod.Spec.NodeName

	case "spec.serviceAccountName":
		return pod.Spec.ServiceAccountName

	case "status.hostIP":
		return pod.Status.HostIP

	case "status.podIP":
		return pod.Status.PodIP

	case "status.podIPs":
		output := make([]string, len(pod.Status.PodIPs))
		for index, ip := range pod.Status.PodIPs {
			output[index] = ip.IP
		}

		return strings.Join(output, ",")

	default:
		return fmt.Sprintf("<`%s` field ref not implemented>", field.FieldPath)
	}
}

func getEnvResourceRef(pod v1.PodSpec, node v1.Node, resource v1.ResourceFieldSelector) string {
	var container v1.Container

	for _, container = range pod.Containers {
		if resource.ContainerName == container.Name {
			break
		}
	}

	switch resource.Resource {
	case "limits.cpu":
		return getResourceLimit(*container.Resources.Limits.Cpu(), *node.Status.Capacity.Cpu(), resource.Divisor)

	case "limits.memory":
		return getResourceLimit(*container.Resources.Limits.Memory(), *node.Status.Capacity.Memory(), resource.Divisor)

	case "limits.ephemeral-storage":
		return getResourceLimit(*container.Resources.Limits.StorageEphemeral(), *node.Status.Capacity.StorageEphemeral(), resource.Divisor)

	case "requests.cpu":
		return getResourceRequest(*container.Resources.Limits.Cpu(), resource.Divisor)

	case "requests.memory":
		return getResourceRequest(*container.Resources.Limits.Memory(), resource.Divisor)

	case "requests.ephemeral-storage":
		return getResourceRequest(*container.Resources.Limits.StorageEphemeral(), resource.Divisor)

	default:
		return ""
	}
}

func getResourceLimit(defined, node, divisor kubeResource.Quantity) string {
	limit := defined.MilliValue()
	if limit == 0 {
		limit = node.MilliValue()
	}

	return fmt.Sprintf("%.0f", math.Ceil(float64(limit)/float64(divisor.MilliValue())))
}

func getResourceRequest(defined, divisor kubeResource.Quantity) string {
	limit := defined.MilliValue()
	if limit == 0 {
		return "0"
	}

	value := limit / divisor.MilliValue()
	if value == 0 {
		return "1"
	}

	return strconv.FormatInt(value, 10)
}
