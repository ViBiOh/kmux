package forward

import (
	"context"
	"sync"
	"testing"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "default"

func newKube(objects ...runtime.Object) client.Kube {
	return client.New("test", testNamespace, nil, fake.NewClientset(objects...))
}

func podWithPorts(ports ...v1.ContainerPort) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: testNamespace},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: "app", Ports: ports}},
		},
	}
}

func TestGetForwardPort(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pod        *v1.Pod
		remotePort string
		want       int32
	}{
		"numeric port is used as is": {
			podWithPorts(),
			"8080",
			8080,
		},
		"named port of a container": {
			podWithPorts(v1.ContainerPort{Name: "http", ContainerPort: 8081}),
			"http",
			8081,
		},
		"unknown name": {
			podWithPorts(v1.ContainerPort{Name: "http", ContainerPort: 8081}),
			"grpc",
			0,
		},
		"no port declared": {
			podWithPorts(),
			"http",
			0,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, getForwardPort(testCase.pod, testCase.remotePort))
		})
	}
}

func TestIsForwardPodReady(t *testing.T) {
	t.Parallel()

	withReadiness := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: testNamespace},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:           "app",
				Ports:          []v1.ContainerPort{{ContainerPort: 8080}},
				ReadinessProbe: &v1.Probe{},
			}},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{{Name: "app", Ready: false}},
		},
	}

	readyPod := withReadiness.DeepCopy()
	readyPod.Status.ContainerStatuses[0].Ready = true

	noStatusPod := withReadiness.DeepCopy()
	noStatusPod.Status.ContainerStatuses = nil

	cases := map[string]struct {
		pod  *v1.Pod
		port int32
		want bool
	}{
		"no readiness probe means ready": {
			podWithPorts(v1.ContainerPort{ContainerPort: 8080}),
			8080,
			true,
		},
		"port of no container means ready": {
			withReadiness,
			9090,
			true,
		},
		"not ready container": {
			withReadiness,
			8080,
			false,
		},
		"ready container": {
			readyPod,
			8080,
			true,
		},
		"missing status": {
			noStatusPod,
			8080,
			false,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, isForwardPodReady(testCase.pod, testCase.port))
		})
	}
}

func TestGetSelectorAndTargetPort(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: testNamespace},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{"app": "web"},
				Ports: []v1.ServicePort{
					{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)},
					{Name: "grpc", Port: 9090, TargetPort: intstr.FromString("grpc-port")},
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: testNamespace},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt32(8080)}},
			},
		},
	}

	cases := map[string]struct {
		name            string
		port            string
		wantPort        string
		wantHasSelector bool
		wantErr         string
	}{
		"by port number": {
			"web", "80",
			"8080", true, "",
		},
		"by port name": {
			"web", "http",
			"8080", true, "",
		},
		"named target port": {
			"web", "grpc",
			"grpc-port", true, "",
		},
		"unknown port is kept": {
			"web", "1234",
			"1234", true, "",
		},
		"service without selector": {
			"headless", "80",
			"8080", false, "",
		},
		"unknown service": {
			"missing", "80",
			"", false, "get service",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			port, hasSelector, err := getSelectorAndTargetPort(context.Background(), newKube(objects...), testCase.name, testCase.port)

			if len(testCase.wantErr) != 0 {
				assert.ErrorContains(t, err, testCase.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantPort, port)
			assert.Equal(t, testCase.wantHasSelector, hasSelector)
		})
	}
}

func TestForwardWithoutPool(t *testing.T) {
	t.Parallel()

	err := NewForwarder("deployment", "web", "8080", nil, 0).Forward(context.Background(), newKube())

	assert.ErrorContains(t, err, "no local pool", "forwarding without a pool must fail instead of panicking")
}

func TestGetFreePort(t *testing.T) {
	t.Parallel()

	port, err := GetFreePort()

	require.NoError(t, err)
	assert.Positive(t, port)
}

func TestForwardSetAdd(t *testing.T) {
	t.Parallel()

	instance := newForwardSet()

	first, ok := instance.add(types.UID("pod-1"))
	require.True(t, ok)
	require.NotNil(t, first)

	_, ok = instance.add(types.UID("pod-1"))
	assert.False(t, ok, "a pod already forwarded is not added again")

	second, ok := instance.add(types.UID("pod-2"))
	require.True(t, ok)
	assert.NotNil(t, second)
}

// TestForwardSetStopIsIdempotent guards the panic on a pod notified twice as
// not ready.
func TestForwardSetStopIsIdempotent(t *testing.T) {
	t.Parallel()

	instance := newForwardSet()

	stopChan, ok := instance.add(types.UID("pod-1"))
	require.True(t, ok)

	assert.NotPanics(t, func() {
		instance.stop(types.UID("pod-1"))
		instance.stop(types.UID("pod-1"))
		instance.stop(types.UID("unknown"))
	})

	_, open := <-stopChan
	assert.False(t, open, "the stop channel is closed")
}

func TestForwardSetStopAll(t *testing.T) {
	t.Parallel()

	instance := newForwardSet()

	first, _ := instance.add(types.UID("pod-1"))
	second, _ := instance.add(types.UID("pod-2"))

	instance.stop(types.UID("pod-1"))

	assert.NotPanics(t, func() {
		instance.stopAll()
		instance.stopAll()
	}, "stopping everything after a single stop must not close a channel twice")

	_, open := <-first
	assert.False(t, open)

	_, open = <-second
	assert.False(t, open)
}

func TestForwardSetConcurrently(t *testing.T) {
	t.Parallel()

	instance := newForwardSet()

	var wg sync.WaitGroup

	for index := range 32 {
		wg.Go(func() {
			pod := types.UID("pod-" + string(rune('a'+index%4)))

			if _, ok := instance.add(pod); ok {
				instance.stop(pod)
			}

			instance.stop(pod)
		})
	}

	wg.Go(func() {
		for range 8 {
			instance.stopAll()
		}
	})

	wg.Wait()
}
