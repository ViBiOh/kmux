package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/concurrent"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/ViBiOh/kmux/pkg/tcpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var portForwardCmd = &cobra.Command{
	Use:     "port-forward TYPE NAME [local_port:]remote_port",
	Aliases: []string{"forward"},
	Short:   "Port forward to pods of a resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"daemonsets",
				"deployments",
				"pods",
				"services",
				"statefulsets",
			}, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			lister, err := resource.ListerFor(args[0])
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			clients, err = getKubernetesClient(strings.Split(viper.GetString("context"), ","))
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			return getCommonObjects(viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.ExactValidArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceType := args[0]
		resourceName := args[1]
		rawPort := args[2]

		ports := strings.SplitN(rawPort, ":", 2)

		localPort, err := strconv.ParseUint(ports[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid local port: %s", ports[0])
		}

		var remotePort string
		if len(ports) == 2 {
			remotePort = ports[1]
		} else {
			remotePort = ports[0]
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		output.Std("", "Listening tcp on %d", localPort)

		var pool *tcpool.Pool
		if !dryRun {
			pool = tcpool.New()
			go pool.Start(ctx, localPort)
		}

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		clients.Execute(func(kube client.Kube) error {
			remotePort := remotePort

			if resource.IsService(resourceType) {
				var err error

				remotePort, err = getTargetPort(ctx, kube, resourceName, remotePort)
				if err != nil {
					kube.Err("get target port: %s", err)
				}
			}

			podWatcher, err := resource.WatchPods(ctx, kube, resourceType, resourceName, dryRun)
			if err != nil {
				return err
			}

			defer podWatcher.Stop()

			var activeForwarding sync.Map

			forwarding := concurrent.NewSimple()

			for event := range podWatcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				remotePort := getForwardPort(pod, remotePort)
				if remotePort == 0 {
					kube.Err("port `%d` not found", remotePort)
					continue
				}

				isContainerReady := isForwardPodReady(pod, int32(remotePort))

				forwardStop, ok := activeForwarding.Load(pod.UID)
				if event.Type == watch.Deleted || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed || !isContainerReady {
					if ok {
						close(forwardStop.(chan struct{}))
					}

					continue
				}

				if ok || pod.Status.Phase != v1.PodRunning || !isContainerReady {
					continue
				}

				handleForwardPod(kube, &activeForwarding, forwarding, *pod, pool, remotePort)
			}

			activeForwarding.Range(func(key, value any) bool {
				close(value.(chan struct{}))
				return true
			})

			forwarding.Wait()

			return nil
		})

		if pool != nil {
			<-pool.Done()
		}

		return nil
	},
}

func initPortForward() {
	flags := portForwardCmd.Flags()

	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
}

func getTargetPort(ctx context.Context, kube client.Kube, name, port string) (string, error) {
	service, err := kube.CoreV1().Services(kube.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get service: %w", err)
	}

	for _, servicePort := range service.Spec.Ports {
		if servicePort.Name == port || string(servicePort.Port) == port {
			return string(servicePort.TargetPort.StrVal), nil
		}
	}

	return port, nil
}

func getForwardPort(pod *v1.Pod, remotePort string) int32 {
	numericPort, err := strconv.ParseUint(remotePort, 10, 32)
	if err == nil {
		return int32(numericPort)
	}

	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == remotePort {
				return port.ContainerPort
			}
		}
	}

	return 0
}

func isForwardPodReady(pod *v1.Pod, remotePort int32) bool {
	container, hasReadiness := getForwardContainer(pod, remotePort)

	if len(container) == 0 {
		return false
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

func handleForwardPod(kube client.Kube, activeForwarding *sync.Map, forwarding *concurrent.Simple, pod v1.Pod, pool *tcpool.Pool, remotePort int32) {
	stopChan := make(chan struct{})
	activeForwarding.Store(pod.UID, stopChan)

	forwarding.Go(func() {
		defer activeForwarding.Delete(pod.UID)

		port, err := GetFreePort()
		if err != nil {
			kube.Err("get free port: %s", err)
			return
		}

		backend := fmt.Sprintf("127.0.0.1:%d", port)

		kube.Std("Forwarding from %s to %s...", output.Blue(backend), output.Green(fmt.Sprintf("%s:%d", pod.Name, remotePort)))
		if dryRun {
			return
		}

		defer kube.Warn("Forwarding to %s ended.", pod.Name)

		pool.Add(backend)
		defer pool.Remove(backend)

		if err := listenPortForward(kube, pod, stopChan, port, remotePort); err != nil {
			kube.Err("Port-forward for %s failed: %s", pod.Name, err)
		}
	})
}

func listenPortForward(kube client.Kube, pod v1.Pod, stopChan chan struct{}, localPort, podPort int32) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", pod.Namespace, pod.Name)
	hostIP := strings.TrimPrefix(kube.Config.Host, "https://")

	transport, upgrader, err := spdy.RoundTripperFor(kube.Config)
	if err != nil {
		return fmt.Errorf("transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, podPort)}, stopChan, nil, nil, kube.Outputter)
	if err != nil {
		return err
	}

	return forwarder.ForwardPorts()
}

func GetFreePort() (int32, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("listen tcp: %s", err)
	}

	if closeErr := listener.Close(); closeErr != nil {
		output.Err("", "close free port listener: %s", err)
	}

	return int32(listener.Addr().(*net.TCPAddr).Port), nil
}
