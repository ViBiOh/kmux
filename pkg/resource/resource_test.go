package resource

import (
	"context"
	"regexp"
	"testing"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "default"

func newKube(objects ...runtime.Object) client.Kube {
	return client.New("test", testNamespace, nil, fake.NewClientset(objects...))
}

func TestIsService(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		want bool
	}{
		"short":    {"svc", true},
		"singular": {"service", true},
		"plural":   {"services", true},
		"other":    {"deployment", false},
		"empty":    {"", false},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, IsService(testCase.name))
		})
	}
}

func TestSelectedContainers(t *testing.T) {
	t.Parallel()

	spec := v1.PodSpec{
		InitContainers: []v1.Container{{Name: "init"}, {Name: "init-config"}},
		Containers:     []v1.Container{{Name: "app"}, {Name: "sidecar"}},
	}

	cases := map[string]struct {
		filter *regexp.Regexp
		want   []string
	}{
		"no filter keeps everything, init containers first": {
			nil,
			[]string{"init", "init-config", "app", "sidecar"},
		},
		"filter on a regular container": {
			regexp.MustCompile("^app$"),
			[]string{"app"},
		},
		"filter on init containers": {
			regexp.MustCompile("^init"),
			[]string{"init", "init-config"},
		},
		"filter matching nothing": {
			regexp.MustCompile("nope"),
			[]string{},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got := SelectedContainers(spec, testCase.filter)

			names := make([]string, 0, len(got))
			for _, container := range got {
				names = append(names, container.Name)
			}

			assert.Equal(t, testCase.want, names)

			assert.Equal(t, []string{"init", "init-config"}, []string{spec.InitContainers[0].Name, spec.InitContainers[1].Name}, "the given spec must not be modified")
			assert.Len(t, spec.Containers, 2, "the given spec must not be modified")
		})
	}
}

func TestListerFor(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: testNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: testNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "kube-system"}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: testNamespace}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web-123", Namespace: testNamespace}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: testNamespace}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "cleaner", Namespace: testNamespace}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "migration", Namespace: testNamespace}},
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-123-abc", Namespace: testNamespace}},
		&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web-svc", Namespace: testNamespace}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}},
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}

	cases := map[string]struct {
		kind string
		want []string
	}{
		"cronjobs":     {"cj", []string{"cleaner"}},
		"daemonsets":   {"daemonset", []string{"agent"}},
		"deployments":  {"deployments", []string{"api", "web"}},
		"jobs":         {"job", []string{"migration"}},
		"pods":         {"po", []string{"web-123-abc"}},
		"replicasets":  {"rs", []string{"web-123"}},
		"statefulsets": {"sts", []string{"db"}},
		"services":     {"svc", []string{"web-svc"}},
		"namespaces":   {"ns", []string{testNamespace}},
		"nodes":        {"no", []string{"node-1"}},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			lister, err := ListerFor(testCase.kind)
			require.NoError(t, err)
			require.NotNil(t, lister)

			got, err := lister(context.Background(), newKube(objects...), testNamespace)
			require.NoError(t, err)

			assert.ElementsMatch(t, testCase.want, got)
		})
	}
}

func TestListerForUnhandled(t *testing.T) {
	t.Parallel()

	lister, err := ListerFor("configmap")

	assert.Nil(t, lister)
	assert.ErrorContains(t, err, "unhandled resource type `configmap`")
}

func TestSelectorFromLabelSelector(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		selector *metav1.LabelSelector
		want     string
		wantErr  string
	}{
		"nil selector": {
			nil,
			"",
			"resource has no selector",
		},
		"match labels": {
			&metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			"app=web",
			"",
		},
		"match expressions are kept": {
			&metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "tier",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"front"},
				}},
			},
			"app=web,tier in (front)",
			"",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got, err := selectorFromLabelSelector(testCase.selector)

			if len(testCase.wantErr) != 0 {
				assert.ErrorContains(t, err, testCase.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestGetPodsSelector(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: testNamespace},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "no-selector", Namespace: testNamespace},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web-svc", Namespace: testNamespace},
			Spec:       v1.ServiceSpec{Selector: map[string]string{"app": "web"}},
		},
	}

	cases := map[string]struct {
		kind              string
		name              string
		wantNamespace     string
		wantLabelSelector string
		wantFieldSelector string
		wantFilter        bool
		wantErr           string
	}{
		"deployment": {
			"deployment", "web",
			testNamespace, "app=web", "", false, "",
		},
		"deployment without selector": {
			"deployment", "no-selector",
			testNamespace, "", "", false, "has no selector",
		},
		"missing deployment": {
			"deployment", "unknown",
			"", "", "", false, "not found",
		},
		"service": {
			"svc", "web-svc",
			testNamespace, "app=web", "", false, "",
		},
		"missing service": {
			"svc", "unknown",
			"", "", "", false, "get services",
		},
		"pod": {
			"pod", "web-123",
			testNamespace, "", "metadata.name=web-123", false, "",
		},
		"node": {
			"node", "node-1",
			testNamespace, "", "spec.nodeName=node-1", false, "",
		},
		"namespace defaults to the context one": {
			"ns", "",
			testNamespace, "", "", false, "",
		},
		"namespace by name": {
			"namespace", "other",
			"other", "", "", false, "",
		},
		"unhandled kind": {
			"configmap", "name",
			"", "", "", false, "unhandled resource type",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			namespace, options, filter, err := GetPodsSelector(context.Background(), newKube(objects...), testCase.kind, testCase.name)

			if len(testCase.wantErr) != 0 {
				assert.ErrorContains(t, err, testCase.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantNamespace, namespace)
			assert.Equal(t, testCase.wantLabelSelector, options.LabelSelector)
			assert.Equal(t, testCase.wantFieldSelector, options.FieldSelector)
			assert.Equal(t, testCase.wantFilter, filter != nil)
		})
	}
}

func TestCronJobPodFilter(t *testing.T) {
	t.Parallel()

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cleaner", Namespace: testNamespace, UID: types.UID("cronjob-uid")},
	}

	owned := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cleaner-1",
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "cleaner", UID: types.UID("cronjob-uid")}},
		},
	}

	foreign := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "other-1",
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "other", UID: types.UID("other-uid")}},
		},
	}

	podOf := func(jobName string) v1.Pod {
		return v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:            jobName + "-abc",
				Namespace:       testNamespace,
				OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: jobName}},
			},
		}
	}

	cases := map[string]struct {
		pod  v1.Pod
		want bool
	}{
		"pod of an owned job": {
			podOf("cleaner-1"),
			true,
		},
		"pod of a foreign job": {
			podOf("other-1"),
			false,
		},
		"pod of an unknown job": {
			podOf("missing"),
			false,
		},
		"pod without owner": {
			v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "standalone", Namespace: testNamespace}},
			false,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			kube := newKube(cronjob, owned, foreign)
			filter := cronJobPodFilter(cronjob)

			assert.Equal(t, testCase.want, filter(context.Background(), kube, testCase.pod))
		})
	}
}

// TestCronJobPodFilterCaches makes sure jobs are resolved once, the filter runs
// for every pod of every event.
func TestCronJobPodFilterCaches(t *testing.T) {
	t.Parallel()

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cleaner", Namespace: testNamespace, UID: types.UID("cronjob-uid")},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cleaner-1",
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "cleaner", UID: types.UID("cronjob-uid")}},
		},
	}

	clientset := fake.NewClientset(cronjob, job)
	kube := client.New("test", testNamespace, nil, clientset)

	filter := cronJobPodFilter(cronjob)

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cleaner-1-abc",
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: "cleaner-1"}},
		},
	}

	for range 5 {
		assert.True(t, filter(context.Background(), kube, pod))
	}

	var jobGets int

	for _, action := range clientset.Actions() {
		if action.Matches("get", "jobs") {
			jobGets++
		}
	}

	assert.Equal(t, 1, jobGets, "the job must be fetched once")
}

func TestListPods(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: testNamespace},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			},
		},
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: testNamespace, Labels: map[string]string{"app": "web"}}},
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-2", Namespace: testNamespace, Labels: map[string]string{"app": "web"}}},
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: testNamespace, Labels: map[string]string{"app": "api"}}},
	}

	pods, err := ListPods(context.Background(), newKube(objects...), "deployment", "web")
	require.NoError(t, err)

	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}

	assert.ElementsMatch(t, []string{"web-1", "web-2"}, names)
}
