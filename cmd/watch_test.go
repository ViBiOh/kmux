package cmd

import (
	"testing"
	"time"

	"github.com/ViBiOh/kmux/pkg/table"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPodStatus(t *testing.T) {
	t.Parallel()

	restartAlways := v1.ContainerRestartPolicyAlways
	terminatedAt := metav1.NewTime(time.Date(2024, time.March, 1, 10, 0, 0, 0, time.UTC))

	cases := map[string]struct {
		pod             v1.Pod
		wantReason      string
		wantReady       uint
		wantTotal       int
		wantRestart     uint
		wantRestartDate time.Time
	}{
		"running and ready": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{{
						Name:  "app",
						Ready: true,
						State: v1.ContainerState{Running: &v1.ContainerStateRunning{}},
					}},
				},
			},
			"Running", 1, 1, 0,
			time.Time{},
		},
		"pending without status": {
			v1.Pod{
				Spec:   v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{Phase: v1.PodPending},
			},
			"Pending", 0, 1, 0,
			time.Time{},
		},
		"status reason wins over the phase": {
			v1.Pod{
				Spec:   v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{Phase: v1.PodPending, Reason: "Evicted"},
			},
			"Evicted", 0, 1, 0,
			time.Time{},
		},
		"crash looping container": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{{
						Name:         "app",
						RestartCount: 3,
						State:        v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
						LastTerminationState: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{ExitCode: 1, FinishedAt: terminatedAt},
						},
					}},
				},
			},
			"CrashLoopBackOff", 0, 1, 3, terminatedAt.Time,
		},
		"terminated without reason": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
					ContainerStatuses: []v1.ContainerStatus{{
						Name:  "app",
						State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 2}},
					}},
				},
			},
			"ExitCode:2", 0, 1, 0,
			time.Time{},
		},
		"killed by a signal": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
					ContainerStatuses: []v1.ContainerStatus{{
						Name:  "app",
						State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Signal: 9}},
					}},
				},
			},
			"Signal:9", 0, 1, 0,
			time.Time{},
		},
		"completed job pod": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
					ContainerStatuses: []v1.ContainerStatus{{
						Name:  "app",
						State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Completed"}},
					}},
				},
			},
			"Completed", 0, 1, 0,
			time.Time{},
		},
		"completed with a container still running and ready": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "job"}, {Name: "sidecar"}}},
				Status: v1.PodStatus{
					Phase:      v1.PodRunning,
					Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "job", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Completed"}}},
						{Name: "sidecar", Ready: true, State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			"Running", 1, 2, 0,
			time.Time{},
		},
		"completed with a container running but not ready": {
			v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Name: "job"}, {Name: "sidecar"}}},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "job", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Completed"}}},
						{Name: "sidecar", Ready: true, State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			"NotReady", 1, 2, 0,
			time.Time{},
		},
		"init container pulling": {
			v1.Pod{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{{Name: "init"}},
					Containers:     []v1.Container{{Name: "app"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					InitContainerStatuses: []v1.ContainerStatus{{
						Name:  "init",
						State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
					}},
				},
			},
			"Init:ImagePullBackOff", 0, 1, 0,
			time.Time{},
		},
		"init container failed": {
			v1.Pod{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{{Name: "init"}},
					Containers:     []v1.Container{{Name: "app"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					InitContainerStatuses: []v1.ContainerStatus{{
						Name:  "init",
						State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 1}},
					}},
				},
			},
			"Init:ExitCode:1", 0, 1, 0,
			time.Time{},
		},
		"init container in progress": {
			v1.Pod{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{{Name: "first"}, {Name: "second"}},
					Containers:     []v1.Container{{Name: "app"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					InitContainerStatuses: []v1.ContainerStatus{{
						Name:  "first",
						State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "PodInitializing"}},
					}},
				},
			},
			"Init:0/2", 0, 1, 0,
			time.Time{},
		},
		"restartable init container counts in the total": {
			v1.Pod{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{{Name: "sidecar", RestartPolicy: &restartAlways}},
					Containers:     []v1.Container{{Name: "app"}},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					InitContainerStatuses: []v1.ContainerStatus{{
						Name:    "sidecar",
						Ready:   true,
						Started: ptr(true),
						State:   v1.ContainerState{Running: &v1.ContainerStateRunning{}},
					}},
					ContainerStatuses: []v1.ContainerStatus{{
						Name:  "app",
						Ready: true,
						State: v1.ContainerState{Running: &v1.ContainerStateRunning{}},
					}},
				},
			},
			"Running", 2, 2, 0,
			time.Time{},
		},
		"terminating pod": {
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr(metav1.Now())},
				Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{{
						Name:  "app",
						Ready: true,
						State: v1.ContainerState{Running: &v1.ContainerStateRunning{}},
					}},
				},
			},
			"Terminating", 1, 1, 0,
			time.Time{},
		},
		"pod of a lost node": {
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr(metav1.Now())},
				Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
				Status:     v1.PodStatus{Phase: v1.PodRunning, Reason: "NodeLost"},
			},
			"Unknown", 0, 1, 0,
			time.Time{},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			reason, ready, total, restart, lastRestartDate := getPodStatus(testCase.pod)

			assert.Equal(t, testCase.wantReason, reason, "reason")
			assert.Equal(t, testCase.wantReady, ready, "ready")
			assert.Equal(t, testCase.wantTotal, total, "total")
			assert.Equal(t, testCase.wantRestart, restart, "restart")
			assert.Equal(t, testCase.wantRestartDate, lastRestartDate.UTC(), "last restart date")
		})
	}
}

func TestGetPodWide(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pod                v1.Pod
		wantIP             string
		wantNode           string
		wantNominated      string
		wantReadinessGates string
	}{
		"empty pod": {
			v1.Pod{},
			noneValue, noneValue, noneValue, noneValue,
		},
		"scheduled pod": {
			v1.Pod{
				Spec:   v1.PodSpec{NodeName: "node-1"},
				Status: v1.PodStatus{PodIPs: []v1.PodIP{{IP: "10.1.0.1"}}},
			},
			"10.1.0.1", "node-1", noneValue, noneValue,
		},
		"nominated node": {
			v1.Pod{Status: v1.PodStatus{NominatedNodeName: "node-2"}},
			noneValue, noneValue, "node-2", noneValue,
		},
		"readiness gates": {
			v1.Pod{
				Spec: v1.PodSpec{ReadinessGates: []v1.PodReadinessGate{
					{ConditionType: "custom/one"},
					{ConditionType: "custom/two"},
				}},
				Status: v1.PodStatus{Conditions: []v1.PodCondition{
					{Type: "custom/one", Status: v1.ConditionTrue},
					{Type: "custom/two", Status: v1.ConditionFalse},
				}},
			},
			noneValue, noneValue, noneValue, "1/2",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			ip, node, nominated, gates := getPodWide(testCase.pod)

			assert.Equal(t, testCase.wantIP, ip)
			assert.Equal(t, testCase.wantNode, node)
			assert.Equal(t, testCase.wantNominated, nominated)
			assert.Equal(t, testCase.wantReadinessGates, gates)
		})
	}
}

func TestGetPhaseCell(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		phase string
		want  string
	}{
		"running":    {"Running", "Running"},
		"failed":     {"CrashLoopBackOff", "CrashLoopBackOff"},
		"pending":    {"Pending", "Pending"},
		"terminated": {"Terminating", "Terminating"},
		"unknown":    {"Whatever", "Whatever"},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got := table.New([]uint64{0}).Format([]table.Cell{getPhaseCell(testCase.phase)})

			assert.Contains(t, got, testCase.want)
		})
	}
}

func TestMapAsString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		values map[string]string
		want   string
	}{
		"empty": {
			nil,
			"",
		},
		"sorted": {
			map[string]string{"zed": "3", "app": "1", "tier": "2"},
			"app=1,tier=2,zed=3",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, mapAsString(testCase.values))
		})
	}
}

func TestPodByAge(t *testing.T) {
	t.Parallel()

	older := metav1.NewTime(time.Date(2024, time.March, 1, 10, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2024, time.March, 2, 10, 0, 0, 0, time.UTC))

	pods := PodByAge{
		{Pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "newer"}, Status: v1.PodStatus{StartTime: &newer}}},
		{Pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "without start time"}}},
		{Pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "older"}, Status: v1.PodStatus{StartTime: &older}}},
	}

	assert.Equal(t, 3, pods.Len())
	assert.True(t, pods.Less(2, 0), "older is before newer")
	assert.False(t, pods.Less(0, 2))
	assert.False(t, pods.Less(1, 0), "a pod without start time is never before")
}

// TestOutputWatchDoesNotPanic is not parallel, it reads the shared output flags.
func TestOutputWatchDoesNotPanic(t *testing.T) {
	watchTable := table.New([]uint64{10, 5, 9, 6, 14})

	assert.NotPanics(t, func() {
		outputWatch(watchTable, "ctx", v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default"},
			Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "app"}}},
			Status:     v1.PodStatus{Phase: v1.PodRunning},
		})
	})
}
