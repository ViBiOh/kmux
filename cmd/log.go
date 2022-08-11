package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/concurrent"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	dryRun       bool
	since        time.Duration
	sinceSeconds int64
	containers   []string
)

var logCmd = &cobra.Command{
	Use:     "log <resource_type> <resource_name>",
	Aliases: []string{"logs"},
	Short:   "Get logs of a given resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return resource.Resources, cobra.ShellCompDirectiveNoFileComp
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
	Args: cobra.ExactValidArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		resourceName := args[1]

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		sinceSeconds = int64(since.Seconds())

		clients.Execute(func(kube client.Kube) error {
			podWatcher, err := resource.GetPodWatcher(resourceType, resourceName)(ctx, kube)
			if err != nil {
				return err
			}

			defer podWatcher.Stop()

			activeStreams := make(map[types.UID]func())

			streaming := concurrent.NewSimple()

			for event := range podWatcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				streamCancel, ok := activeStreams[pod.UID]

				if event.Type == watch.Deleted || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
					if ok {
						streamCancel()
						delete(activeStreams, pod.UID)
					} else if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
						handlePod(ctx, activeStreams, streaming, kube, pod)
					}

					continue
				}

				if ok || pod.Status.Phase == v1.PodPending {
					continue
				}

				handlePod(ctx, activeStreams, streaming, kube, pod)
			}

			streaming.Wait()

			return nil
		})
	},
}

func initLog() {
	flags := logCmd.Flags()

	flags.DurationVarP(&since, "since", "s", time.Hour, "Display logs since given duration")
	flags.StringSliceVarP(&containers, "containers", "c", nil, "Filter container's name, default to all containers")
	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
}

func handlePod(ctx context.Context, activeStreams map[types.UID]func(), streaming *concurrent.Simple, kube client.Kube, pod *v1.Pod) {
	for _, container := range pod.Spec.Containers {
		if !isContainerSelected(container) {
			continue
		}

		container := container

		if dryRun {
			kube.Info("%s %s", output.Green(fmt.Sprintf("[%s/%s]", pod.Name, container.Name)), output.Yellow("Found!"))
			continue
		}

		streaming.Go(func() {
			if pod.Status.Phase != v1.PodRunning {
				logPod(ctx, kube, pod.Namespace, pod.Name, container.Name)
				return
			}

			streamCtx, streamCancel := context.WithCancel(ctx)
			activeStreams[pod.UID] = streamCancel
			defer streamCancel()

			streamPod(streamCtx, kube, pod.Namespace, pod.Name, container.Name)
		})
	}
}

func logPod(ctx context.Context, kube client.Kube, namespace, name, container string) {
	content, err := kube.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		SinceSeconds: &sinceSeconds,
		Container:    container,
	}).DoRaw(ctx)
	if err != nil {
		kube.Err("%s", err)
		return
	}

	outputLog(bytes.NewReader(content), kube, name, container)
}

func streamPod(ctx context.Context, kube client.Kube, namespace, name, container string) {
	stream, err := kube.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Follow:       true,
		SinceSeconds: &sinceSeconds,
		Container:    container,
	}).Stream(ctx)
	if err != nil {
		kube.Err("%s", err)
		return
	}

	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			kube.Err("close stream: %s", closeErr)
		}
	}()

	outputLog(stream, kube, name, container)
}

func outputLog(reader io.Reader, kube client.Kube, name, container string) {
	outputter := kube.Child(output.Green(fmt.Sprintf("[%s/%s]", name, container)))

	outputter.Info(output.Yellow("Starting log..."))
	defer outputter.Info(output.Yellow("Log ended."))

	streamScanner := bufio.NewScanner(reader)
	streamScanner.Split(bufio.ScanLines)

	for streamScanner.Scan() {
		outputter.Std(streamScanner.Text())
	}
}

func isContainerSelected(container v1.Container) bool {
	if len(containers) == 0 {
		return true
	}

	for _, containerName := range containers {
		if strings.EqualFold(containerName, container.Name) {
			return true
		}
	}

	return false
}
