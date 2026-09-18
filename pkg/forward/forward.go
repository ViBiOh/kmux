package forward

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/ViBiOh/kmux/pkg/tcpool"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type Forwarder struct {
	pool       *tcpool.Pool
	kind       string
	name       string
	remotePort string
	limiter    uint
	dryRun     bool
}

func NewForwarder(kind, name, remotePort string, pool *tcpool.Pool, limiter uint) Forwarder {
	return Forwarder{
		kind:       kind,
		name:       name,
		remotePort: remotePort,
		pool:       pool,
		limiter:    limiter,
	}
}

func (f Forwarder) WithDryRun(dryRun bool) Forwarder {
	f.dryRun = dryRun

	return f
}

func (f Forwarder) Forward(ctx context.Context, kube client.Kube) error {
	if !f.dryRun && f.pool == nil {
		return errors.New("no local pool to forward to")
	}

	remotePort := f.remotePort

	if resource.IsService(f.kind) {
		var hasSelector bool
		var err error

		remotePort, hasSelector, err = getSelectorAndTargetPort(ctx, kube, f.name, remotePort)
		if err != nil {
			return fmt.Errorf("get selector and target port: %w", err)
		}

		if !hasSelector {
			return errors.New("service has no selector")
		}
	}

	podWatcher, err := resource.WatchPods(ctx, kube, f.kind, f.name, nil, f.dryRun)
	if err != nil {
		return err
	}
	defer podWatcher.Stop()

	var podLimiter chan struct{}
	if f.limiter > 0 {
		podLimiter = make(chan struct{}, f.limiter)
		defer close(podLimiter)
	}

	active := newForwardSet()

	var forwarding sync.WaitGroup

	for event := range podWatcher.ResultChan() {
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}

		podPort := getForwardPort(pod, remotePort)
		if podPort == 0 {
			kube.Err("port `%s` not found on pod %s", remotePort, pod.Name)

			continue
		}

		isContainerReady := isForwardPodReady(pod, podPort)
		isGone := event.Type == watch.Deleted || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed

		if isGone || !isContainerReady {
			active.stop(pod.UID)

			continue
		}

		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		f.handleForwardPod(kube, active, &forwarding, *pod, podPort, podLimiter)
	}

	active.stopAll()

	forwarding.Wait()

	return nil
}

func getSelectorAndTargetPort(ctx context.Context, kube client.Kube, name, port string) (string, bool, error) {
	service, err := kube.CoreV1().Services(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", false, fmt.Errorf("get service: %w", err)
	}

	hasSelector := len(service.Spec.Selector) != 0

	for _, servicePort := range service.Spec.Ports {
		if servicePort.Name == port || strconv.Itoa(int(servicePort.Port)) == port {
			if len(servicePort.TargetPort.StrVal) != 0 {
				return servicePort.TargetPort.StrVal, hasSelector, nil
			}

			return strconv.Itoa(int(servicePort.TargetPort.IntVal)), hasSelector, nil
		}
	}

	return port, hasSelector, nil
}

func getForwardPort(pod *v1.Pod, remotePort string) int32 {
	numericPort, err := strconv.ParseInt(remotePort, 10, 32)
	if err == nil {
		return int32(numericPort)
	}

	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == remotePort {
				return port.ContainerPort
			}

			if strconv.Itoa(int(port.ContainerPort)) == remotePort {
				return port.ContainerPort
			}
		}
	}

	return 0
}

func isForwardPodReady(pod *v1.Pod, remotePort int32) bool {
	container, hasReadiness := getForwardContainer(pod, remotePort)

	if len(container) == 0 {
		return true
	}

	if !hasReadiness {
		return true
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container {
			return status.Ready
		}
	}

	return false
}

func getForwardContainer(pod *v1.Pod, remotePort int32) (string, bool) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.ContainerPort == remotePort {
				return container.Name, container.ReadinessProbe != nil
			}
		}
	}

	return "", false
}

func (f Forwarder) handleForwardPod(kube client.Kube, active *forwardSet, forwarding *sync.WaitGroup, pod v1.Pod, remotePort int32, podLimiter chan struct{}) {
	stopChan, ok := active.add(pod.UID)
	if !ok {
		return
	}

	forwarding.Go(func() {
		defer active.stop(pod.UID)

		if podLimiter != nil {
			select {
			case podLimiter <- struct{}{}:
				defer func() { <-podLimiter }()
			case <-stopChan:
				return
			}
		}

		port, err := GetFreePort()
		if err != nil {
			kube.Err("get free port: %s", err)
			return
		}

		backend := fmt.Sprintf("127.0.0.1:%d", port)

		kube.Std("Forwarding from %s to %s...", output.Blue.Sprint(backend), output.Green.Sprintf("%s:%d", pod.Name, remotePort))
		if f.dryRun {
			return
		}

		defer kube.Warn("Forwarding to %s ended.", pod.Name)

		f.pool.Add(backend)
		defer f.pool.Remove(backend)

		if err := listenPortForward(kube, pod, stopChan, port, remotePort); err != nil {
			kube.Err("Port-forward for %s failed: %s", pod.Name, err)
		}
	})
}

func listenPortForward(kube client.Kube, pod v1.Pod, stopChan chan struct{}, localPort, podPort int32) error {
	transport, upgrader, err := spdy.RoundTripperFor(kube.Config)
	if err != nil {
		return fmt.Errorf("transport: %w", err)
	}

	// the request builds the URL from the config, keeping the scheme and any API path prefix
	request := kube.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, request.URL())
	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, podPort)}, stopChan, nil, nil, kube.Outputter)
	if err != nil {
		return err
	}

	return forwarder.ForwardPorts()
}

func GetFreePort() (int32, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("resolve local address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("listen tcp: %w", err)
	}

	if closeErr := listener.Close(); closeErr != nil {
		return 0, fmt.Errorf("close listener: %w", closeErr)
	}

	return int32(listener.Addr().(*net.TCPAddr).Port), nil
}
