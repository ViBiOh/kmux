package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Get all pods in the namespace",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		clients.Execute(func(kube client.Kube) error {
			watcher, err := resource.GetPodWatcher("namespace", kube.Namespace)(ctx, kube)
			if err != nil {
				return err
			}

			defer watcher.Stop()

			line := fmt.Sprintf("%-50s %s %-12s %-6s %s", "NAME", "READY", "PHASE", "AGE", "RESTARTS")
			if allNamespace {
				line = fmt.Sprintf("%-20s %s", "NAMESPACE", line)
			}

			kube.Std(line)

			for event := range watcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				phase, ready, restart, lastRestartDate := getPodStatus(pod)

				var since string
				if pod.Status.StartTime != nil {
					since = duration.HumanDuration(time.Since(pod.Status.StartTime.Time))
				}

				line = fmt.Sprintf("%-50s (%d/%d) %-12s %-6s", pod.Name, ready, len(pod.Status.ContainerStatuses), phase, since)

				switch phase {
				case string(v1.PodRunning), string(v1.PodSucceeded):
					line = output.Green(line)
				case string(v1.PodFailed):
					line = output.Red(line)
				case string(v1.PodPending), "ContainerCreating":
					line = output.Cyan(line)
				case "Terminating":
					line = output.Blue(line)
				default:
					line = output.Yellow(line)
				}

				if restart > 0 {
					line += output.Magenta(fmt.Sprintf(" %d (%s ago)", restart, duration.HumanDuration(time.Since(lastRestartDate))))
				}

				if allNamespace {
					line = fmt.Sprintf("%-20s %s", pod.Namespace, line)
				}

				kube.Std(line)
			}

			return nil
		})
	},
}

// from https://github.com/kubernetes/kubernetes/blob/v1.24.3/pkg/printers/internalversion/printers.go#L799
func getPodStatus(pod *v1.Pod) (string, uint, uint, time.Time) {
	var ready uint
	var restart uint
	lastRestartDate := metav1.NewTime(time.Time{})

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]

		restart += uint(container.RestartCount)

		if container.LastTerminationState.Terminated != nil {
			terminatedDate := container.LastTerminationState.Terminated.FinishedAt
			if lastRestartDate.Before(&terminatedDate) {
				lastRestartDate = terminatedDate
			}
		}

		if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
			reason = container.State.Waiting.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
			reason = container.State.Terminated.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
			if container.State.Terminated.Signal != 0 {
				reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
			} else {
				reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
			}
		} else if container.Ready && container.State.Running != nil {
			ready++
		}
	}

	if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason, ready, restart, lastRestartDate.Time
}
