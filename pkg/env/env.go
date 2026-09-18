package env

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	header := output.Yellow.Sprintf("# %s\n", ev.source)

	entries := make([]string, 0, len(ev.data))

	for key, value := range ev.data {
		entries = append(entries, fmt.Sprintf("%s=%s\n", key, value))
	}

	sort.Strings(entries)

	return header + strings.Join(entries, "")
}

type EnvGetter struct {
	containerRegexp *regexp.Regexp
	kind            string
	name            string
}

func NewEnvGetter(kind, name string) EnvGetter {
	return EnvGetter{
		kind: kind,
		name: name,
	}
}

func (eg EnvGetter) WithContainerRegexp(containerRegexp *regexp.Regexp) EnvGetter {
	eg.containerRegexp = containerRegexp

	return eg
}

func (eg EnvGetter) Get(ctx context.Context, kube client.Kube) error {
	podSpec, err := resource.GetPodSpec(ctx, kube, eg.kind, eg.name)
	if err != nil {
		return err
	}

	pods, err := resource.ListPods(ctx, kube, eg.kind, eg.name)
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

	containers := resource.SelectedContainers(podSpec, eg.containerRegexp)

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
			outputter = kube.Child(false, output.Green.Sprintf("[%s]", container.Name))
		}

		outputter.Std("%s", containerOutput.String())
	}

	return nil
}

// phaseRanks orders phases from the most to the least useful to read live
// values from, an unknown phase ranks last.
var phaseRanks = map[v1.PodPhase]int{
	v1.PodRunning:   0,
	v1.PodSucceeded: 1,
	v1.PodFailed:    2,
	v1.PodPending:   3,
	v1.PodUnknown:   4,
}

func getMostLivePod(pods []v1.Pod) v1.Pod {
	best := v1.Pod{}
	bestRank := len(phaseRanks)

	for _, pod := range pods {
		rank, ok := phaseRanks[pod.Status.Phase]
		if !ok || rank >= bestRank {
			continue
		}

		best, bestRank = pod, rank
	}

	return best
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
			inline[env.Name] = getInlineEnv(pod, container, node, env, configMaps, secrets)
		}

		output = append(output, envValue{
			source: "inline",
			data:   inline,
		})
	}

	return output
}

// getEnvDependencies fetches every configmap and secret referenced by the
// container. Results are gathered in dedicated maps, the requested names are
// only read while fanning out.
func getEnvDependencies(ctx context.Context, kube client.Kube, container v1.Container) (map[string]map[string]string, map[string]map[string]string) {
	wantedConfigMaps, wantedSecrets := gatherEnvDependencies(container)

	configMaps := make(map[string]map[string]string, len(wantedConfigMaps))
	secrets := make(map[string]map[string]string, len(wantedSecrets))

	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, name := range wantedConfigMaps {
		wg.Go(func() {
			configMap, err := kube.CoreV1().ConfigMaps(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				kube.Err("getting configmap `%s`: %s", name, err)

				return
			}

			mutex.Lock()
			defer mutex.Unlock()

			configMaps[name] = configMap.Data
		})
	}

	for _, name := range wantedSecrets {
		wg.Go(func() {
			secret, err := kube.CoreV1().Secrets(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				kube.Err("getting secret `%s`: %s", name, err)

				return
			}

			data := make(map[string]string, len(secret.Data))
			for key, value := range secret.Data {
				data[key] = string(value)
			}

			mutex.Lock()
			defer mutex.Unlock()

			secrets[name] = data
		})
	}

	wg.Wait()

	return configMaps, secrets
}

// gatherEnvDependencies returns the deduplicated names of the configmaps and
// secrets a container reads its environment from.
func gatherEnvDependencies(container v1.Container) ([]string, []string) {
	configMaps := make(map[string]struct{})
	secrets := make(map[string]struct{})

	for _, env := range container.Env {
		if env.ValueFrom == nil {
			continue
		}

		if env.ValueFrom.ConfigMapKeyRef != nil {
			configMaps[env.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
		} else if env.ValueFrom.SecretKeyRef != nil {
			secrets[env.ValueFrom.SecretKeyRef.Name] = struct{}{}
		}
	}

	for _, envFrom := range container.EnvFrom {
		if envFrom.ConfigMapRef != nil {
			configMaps[envFrom.ConfigMapRef.Name] = struct{}{}
		} else if envFrom.SecretRef != nil {
			secrets[envFrom.SecretRef.Name] = struct{}{}
		}
	}

	return sortedKeys(configMaps), sortedKeys(secrets)
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func getEnvFromSource(storage map[string]map[string]string, kind, prefix, name string, optional *bool) (string, map[string]string) {
	content := make(map[string]string)
	keyName := fmt.Sprintf("%s %s", kind, name)

	values, ok := storage[name]
	if !ok {
		if isRequired(optional) {
			content["error"] = fmt.Sprintf("<%s not optional and not found>", kind)

			return keyName, content
		}
	}

	for key, value := range values {
		content[prefix+key] = value
	}

	return keyName, content
}

func getInlineEnv(pod v1.Pod, container v1.Container, node v1.Node, envVar v1.EnvVar, configMaps, secrets map[string]map[string]string) string {
	if len(envVar.Value) > 0 {
		return envVar.Value
	}

	return getValueFrom(pod, container, node, envVar, configMaps, secrets)
}

func getValueFrom(pod v1.Pod, container v1.Container, node v1.Node, envVar v1.EnvVar, configMaps, secrets map[string]map[string]string) string {
	if envVar.ValueFrom == nil {
		return ""
	}

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
		return getEnvResourceRef(container, node, *envVar.ValueFrom.ResourceFieldRef)
	}

	return ""
}

func getValueFromRef(storage map[string]map[string]string, kind, name, key string, optional *bool) string {
	values, ok := storage[name]
	if !ok {
		if isRequired(optional) {
			return fmt.Sprintf("<%s `%s` not optional and not found>", kind, name)
		}

		return ""
	}

	return values[key]
}

// isRequired reports whether a reference must exist, an unset `optional` means
// required for kubernetes.
func isRequired(optional *bool) bool {
	return optional == nil || !*optional
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

func getEnvResourceRef(container v1.Container, node v1.Node, resource v1.ResourceFieldSelector) string {
	switch resource.Resource {
	case "limits.cpu":
		if container.Resources.Limits == nil || node.Status.Capacity == nil {
			return ""
		}
		return getResourceLimit(*container.Resources.Limits.Cpu(), *node.Status.Capacity.Cpu(), resource.Divisor)

	case "limits.memory":
		if container.Resources.Limits == nil || node.Status.Capacity == nil {
			return ""
		}
		return getResourceLimit(*container.Resources.Limits.Memory(), *node.Status.Capacity.Memory(), resource.Divisor)

	case "limits.ephemeral-storage":
		if container.Resources.Limits == nil || node.Status.Capacity == nil {
			return ""
		}
		return getResourceLimit(*container.Resources.Limits.StorageEphemeral(), *node.Status.Capacity.StorageEphemeral(), resource.Divisor)

	case "requests.cpu":
		if container.Resources.Requests == nil {
			return ""
		}
		return getResourceRequest(*container.Resources.Requests.Cpu(), resource.Divisor)

	case "requests.memory":
		if container.Resources.Requests == nil {
			return ""
		}
		return getResourceRequest(*container.Resources.Requests.Memory(), resource.Divisor)

	case "requests.ephemeral-storage":
		if container.Resources.Requests == nil {
			return ""
		}
		return getResourceRequest(*container.Resources.Requests.StorageEphemeral(), resource.Divisor)

	default:
		return ""
	}
}

func getResourceLimit(defined, node, divisor kubeResource.Quantity) string {
	limit := defined.MilliValue()
	if limit == 0 {
		limit = node.MilliValue()
	}

	divider := divisor.MilliValue()
	if divider == 0 {
		divider = 1000
	}

	return fmt.Sprintf("%.0f", math.Ceil(float64(limit)/float64(divider)))
}

func getResourceRequest(defined, divisor kubeResource.Quantity) string {
	limit := defined.MilliValue()
	if limit == 0 {
		return "0"
	}

	divider := divisor.MilliValue()
	if divider == 0 {
		divider = 1000
	}

	value := limit / divider
	if value == 0 {
		return "1"
	}

	return strconv.FormatInt(value, 10)
}
