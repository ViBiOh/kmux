package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/ViBiOh/kube/pkg/output"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

var since time.Duration

var logCmd = &cobra.Command{
	Use:     "log <resource_type> <resource_name>",
	Aliases: []string{"logs"},
	Short:   "Get logs of a given resource",
	Args:    cobra.ExactValidArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ressourceType := args[0]
		resourceName := args[1]

		var labelGetter func(context.Context, kubeClient, string) (string, error)

		switch ressourceType {
		case "deploy", "deployment", "deployments":
			labelGetter = getDeploymentLabelSelector
		case "cj", "cronjob", "cronjobs":
			labelGetter = getCronJobLabelSelector
		case "job", "jobs":
			labelGetter = getJobLabelSelector
		case "ds", "daemonset", "daemonsets":
			labelGetter = getDaemonSetLabelSelector
		default:
			output.Fatal("unhandled resource type for log: %s", ressourceType)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		clients.execute(func(contextName string, client kubeClient) error {
			labelSelector, err := labelGetter(ctx, client, resourceName)
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

func initLog() {
	flags := logCmd.Flags()

	flags.DurationVarP(&since, "since", "s", time.Hour, "Display logs since given duration")
}

func getDeploymentLabelSelector(ctx context.Context, client kubeClient, name string) (string, error) {
	deployment, err := client.clientset.AppsV1().Deployments(client.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(deployment.Spec.Selector), nil
}

func getDaemonSetLabelSelector(ctx context.Context, client kubeClient, name string) (string, error) {
	deployment, err := client.clientset.AppsV1().DaemonSets(client.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(deployment.Spec.Selector), nil
}

func getCronJobLabelSelector(ctx context.Context, client kubeClient, name string) (string, error) {
	job, err := client.clientset.BatchV1().CronJobs(client.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(job.Spec.JobTemplate.Spec.Selector), nil
}

func getJobLabelSelector(ctx context.Context, client kubeClient, name string) (string, error) {
	job, err := client.clientset.BatchV1().Jobs(client.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return toLabelSelector(job.Spec.Selector), nil
}

func toLabelSelector(selector *metav1.LabelSelector) string {
	var labelSelector strings.Builder

	for key, value := range selector.MatchLabels {
		if labelSelector.Len() > 0 {
			labelSelector.WriteString(",")
		}
		labelSelector.WriteString(fmt.Sprintf("%s=%s", key, value))
	}

	return labelSelector.String()
}

func streamPod(ctx context.Context, client kubeClient, contextName, namespace, name, container string) {
	sinceSeconds := int64(since.Seconds())

	stream, err := client.clientset.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Follow:       true,
		SinceSeconds: &sinceSeconds,
	}).Stream(ctx)
	if err != nil {
		output.Err(contextName, "%s", err)
		return
	}

	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			output.Err(contextName, "close stream: %s", closeErr)
		}
	}()

	streamScanner := bufio.NewScanner(stream)
	streamScanner.Split(bufio.ScanLines)

	prefix := output.Green(fmt.Sprintf("[%s/%s]", name, container))

	for streamScanner.Scan() {
		output.Std(contextName, "%s %s", prefix, streamScanner.Text())
	}

	output.StdErr(contextName, "%s %s", prefix, output.Yellow("Stream ended."))
}
