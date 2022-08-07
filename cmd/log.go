package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"logs"},
	Short:   "Get logs of a given resources",
	Args:    cobra.ExactValidArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// ressourceType := args[0]
		resourceName := args[1]

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		clients.execute(func(contextName string, client kubeClient) error {
			labelSelector, err := getDeploymentLabelSelector(ctx, client, resourceName)
			if err != nil {
				return err
			}

			podsWatcher, err := client.clientset.CoreV1().Pods(client.namespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
				Watch:         true,
			})
			if err != nil {
				return err
			}

			defer podsWatcher.Stop()

			onGoingStreams := make(map[types.UID]func())

			streaming := newConcurrent()

			for event := range podsWatcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				if pod.Status.Phase == v1.PodPending {
					continue
				}

				streamCancel, ok := onGoingStreams[pod.UID]

				if event.Type == watch.Deleted || pod.Status.Phase == v1.PodSucceeded {
					if ok {
						streamCancel()
						delete(onGoingStreams, pod.UID)
					}
					continue
				}

				if ok {
					continue
				}

				for _, container := range pod.Spec.Containers {
					streamCtx, streamCancel := context.WithCancel(ctx)
					onGoingStreams[pod.UID] = streamCancel
					container := container

					streaming.run(func() {
						defer streamCancel()

						streamPod(streamCtx, client, contextName, pod.Namespace, pod.Name, container.Name)
					})
				}
			}

			streaming.wait()

			return nil
		})
	},
}

func getDeploymentLabelSelector(ctx context.Context, client kubeClient, name string) (string, error) {
	deployment, err := client.clientset.AppsV1().Deployments(client.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var labelSelector strings.Builder
	for key, value := range deployment.Spec.Selector.MatchLabels {
		if labelSelector.Len() > 0 {
			labelSelector.WriteString(",")
		}
		labelSelector.WriteString(fmt.Sprintf("%s=%s", key, value))
	}

	return labelSelector.String(), nil
}

func streamPod(ctx context.Context, client kubeClient, contextName, namespace, name, container string) {
	stream, err := client.clientset.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Follow: true,
	}).Stream(ctx)
	if err != nil {
		outputErr(contextName, "%s", err)
		return
	}

	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			outputErr(contextName, "close stream: %s", closeErr)
		}
	}()

	streamScanner := bufio.NewScanner(stream)
	streamScanner.Split(bufio.ScanLines)

	prefix := green(fmt.Sprintf("[%s/%s]", name, container))

	for streamScanner.Scan() {
		outputStd(contextName, "%s %s", prefix, streamScanner.Text())
	}

	outputStdErr(contextName, "%s %s", prefix, yellow("Stream ended."))
}
