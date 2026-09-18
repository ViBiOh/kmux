package env

import (
	"context"
	"testing"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	kubeResource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "default"

func newKube(objects ...runtime.Object) client.Kube {
	return client.New("test", testNamespace, nil, fake.NewClientset(objects...))
}

func TestGetMostLivePod(t *testing.T) {
	t.Parallel()

	podIn := func(name string, phase v1.PodPhase) v1.Pod {
		return v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status:     v1.PodStatus{Phase: phase},
		}
	}

	cases := map[string]struct {
		pods []v1.Pod
		want string
	}{
		"no pod": {
			nil,
			"",
		},
		"running is preferred": {
			[]v1.Pod{podIn("pending", v1.PodPending), podIn("running", v1.PodRunning)},
			"running",
		},
		"succeeded over failed": {
			[]v1.Pod{podIn("failed", v1.PodFailed), podIn("succeeded", v1.PodSucceeded)},
			"succeeded",
		},
		"failed over pending": {
			[]v1.Pod{podIn("pending", v1.PodPending), podIn("failed", v1.PodFailed)},
			"failed",
		},
		"pending over unknown": {
			[]v1.Pod{podIn("unknown", v1.PodUnknown), podIn("pending", v1.PodPending)},
			"pending",
		},
		"first of the same phase": {
			[]v1.Pod{podIn("first", v1.PodRunning), podIn("second", v1.PodRunning)},
			"first",
		},
		"unhandled phase is ignored": {
			[]v1.Pod{podIn("weird", v1.PodPhase("Sleeping"))},
			"",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, getMostLivePod(testCase.pods).Name)
		})
	}
}

func TestGatherEnvDependencies(t *testing.T) {
	t.Parallel()

	container := v1.Container{
		Env: []v1.EnvVar{
			{Name: "PLAIN", Value: "value"},
			{Name: "FROM_CM", ValueFrom: &v1.EnvVarSource{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "config"}, Key: "key"}}},
			{Name: "FROM_CM_AGAIN", ValueFrom: &v1.EnvVarSource{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "config"}, Key: "other"}}},
			{Name: "FROM_SECRET", ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "secret"}, Key: "key"}}},
		},
		EnvFrom: []v1.EnvFromSource{
			{ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "bulk-config"}}},
			{SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "secret"}}},
		},
	}

	configMaps, secrets := gatherEnvDependencies(container)

	assert.Equal(t, []string{"bulk-config", "config"}, configMaps, "names are deduplicated and sorted")
	assert.Equal(t, []string{"secret"}, secrets)
}

func TestGetEnvDependencies(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: testNamespace},
			Data:       map[string]string{"key": "value"},
		},
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: testNamespace},
			Data:       map[string][]byte{"password": []byte("s3cret")},
		},
	}

	container := v1.Container{
		EnvFrom: []v1.EnvFromSource{
			{ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "config"}}},
			{ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "missing"}}},
			{SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "secret"}}},
		},
	}

	configMaps, secrets := getEnvDependencies(context.Background(), newKube(objects...), container)

	assert.Equal(t, map[string]map[string]string{"config": {"key": "value"}}, configMaps)
	assert.Equal(t, map[string]map[string]string{"secret": {"password": "s3cret"}}, secrets)
}

func TestGetEnvFromSource(t *testing.T) {
	t.Parallel()

	storage := map[string]map[string]string{
		"config": {"KEY": "value"},
	}

	cases := map[string]struct {
		name     string
		prefix   string
		optional *bool
		wantKey  string
		wantData map[string]string
	}{
		"found": {
			"config", "", nil,
			"configmap config",
			map[string]string{"KEY": "value"},
		},
		"prefixed": {
			"config", "APP_", nil,
			"configmap config",
			map[string]string{"APP_KEY": "value"},
		},
		"missing and required by default": {
			"missing", "", nil,
			"configmap missing",
			map[string]string{"error": "<configmap not optional and not found>"},
		},
		"missing and explicitly required": {
			"missing", "", new(false),
			"configmap missing",
			map[string]string{"error": "<configmap not optional and not found>"},
		},
		"missing and optional": {
			"missing", "", new(true),
			"configmap missing",
			map[string]string{},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			key, content := getEnvFromSource(storage, "configmap", testCase.prefix, testCase.name, testCase.optional)

			assert.Equal(t, testCase.wantKey, key)
			assert.Equal(t, testCase.wantData, content)
		})
	}
}

func TestGetValueFromRef(t *testing.T) {
	t.Parallel()

	storage := map[string]map[string]string{
		"secret": {"password": "s3cret"},
	}

	cases := map[string]struct {
		name     string
		key      string
		optional *bool
		want     string
	}{
		"found": {
			"secret", "password", nil,
			"s3cret",
		},
		"unknown key": {
			"secret", "missing", nil,
			"",
		},
		"missing and required by default": {
			"missing", "password", nil,
			"<secret `missing` not optional and not found>",
		},
		"missing and optional": {
			"missing", "password", new(true),
			"",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, getValueFromRef(storage, "secret", testCase.name, testCase.key, testCase.optional))
		})
	}
}

func TestGetEnvFieldRef(t *testing.T) {
	t.Parallel()

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web-1",
			Namespace:   testNamespace,
			Labels:      map[string]string{"app": "web"},
			Annotations: map[string]string{"owner": "team"},
		},
		Spec: v1.PodSpec{
			NodeName:           "node-1",
			ServiceAccountName: "web-sa",
		},
		Status: v1.PodStatus{
			HostIP: "10.0.0.1",
			PodIP:  "10.1.0.1",
			PodIPs: []v1.PodIP{{IP: "10.1.0.1"}, {IP: "fd00::1"}},
		},
	}

	cases := map[string]struct {
		fieldPath string
		want      string
	}{
		"name":            {"metadata.name", "web-1"},
		"namespace":       {"metadata.namespace", testNamespace},
		"node name":       {"spec.nodeName", "node-1"},
		"service account": {"spec.serviceAccountName", "web-sa"},
		"host ip":         {"status.hostIP", "10.0.0.1"},
		"pod ip":          {"status.podIP", "10.1.0.1"},
		"pod ips":         {"status.podIPs", "10.1.0.1,fd00::1"},
		"label":           {"metadata.labels['app']", "web"},
		"label unquoted":  {"metadata.labels[app]", "web"},
		"unknown label":   {"metadata.labels['nope']", ""},
		"annotation":      {"metadata.annotations['owner']", "team"},
		"not implemented": {"status.phase", "<`status.phase` field ref not implemented>"},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got := getEnvFieldRef(pod, v1.ObjectFieldSelector{FieldPath: testCase.fieldPath})

			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestGetEnvResourceRef(t *testing.T) {
	t.Parallel()

	container := v1.Container{
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    kubeResource.MustParse("2"),
				v1.ResourceMemory: kubeResource.MustParse("1Gi"),
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    kubeResource.MustParse("500m"),
				v1.ResourceMemory: kubeResource.MustParse("512Mi"),
			},
		},
	}

	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    kubeResource.MustParse("8"),
				v1.ResourceMemory: kubeResource.MustParse("16Gi"),
			},
		},
	}

	cases := map[string]struct {
		container v1.Container
		resource  string
		divisor   kubeResource.Quantity
		want      string
	}{
		"cpu limit": {
			container, "limits.cpu",
			kubeResource.Quantity{},
			"2",
		},
		"memory limit": {
			container, "limits.memory", kubeResource.MustParse("1Mi"),
			"1024",
		},
		"cpu request": {
			container, "requests.cpu",
			kubeResource.Quantity{},
			"1",
		},
		"memory request": {
			container, "requests.memory", kubeResource.MustParse("1Mi"),
			"512",
		},
		"cpu limit falls back on the node": {
			v1.Container{Resources: v1.ResourceRequirements{Limits: v1.ResourceList{v1.ResourceMemory: kubeResource.MustParse("1Gi")}}},
			"limits.cpu",
			kubeResource.Quantity{},
			"8",
		},
		"without limits": {
			v1.Container{},
			"limits.cpu",
			kubeResource.Quantity{},
			"",
		},
		"without requests": {
			v1.Container{},
			"requests.cpu",
			kubeResource.Quantity{},
			"",
		},
		"unknown resource": {
			container, "limits.gpu",
			kubeResource.Quantity{},
			"",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got := getEnvResourceRef(testCase.container, node, v1.ResourceFieldSelector{
				Resource: testCase.resource,
				Divisor:  testCase.divisor,
			})

			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestEnvValueString(t *testing.T) {
	t.Parallel()

	instance := envValue{
		source: "inline",
		data: map[string]string{
			"ZED":  "last",
			"ABC":  "first",
			"MIDD": "middle",
		},
	}

	assert.Contains(t, instance.String(), "inline")
	assert.Equal(t, "ABC=first\nMIDD=middle\nZED=last\n", stripHeader(instance.String()))
}

func stripHeader(value string) string {
	for index := range value {
		if value[index] == '\n' {
			return value[index+1:]
		}
	}

	return value
}

func TestGet(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "standalone", Namespace: testNamespace},
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name: "app",
					Env: []v1.EnvVar{
						{Name: "PLAIN", Value: "value"},
						{Name: "NODE", ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
					},
				}},
				NodeName: "node-1",
			},
			Status: v1.PodStatus{Phase: v1.PodRunning},
		},
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}

	err := NewEnvGetter("pod", "standalone").Get(context.Background(), newKube(objects...))

	require.NoError(t, err)
}
