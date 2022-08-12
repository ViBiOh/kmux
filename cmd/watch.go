package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"syscall"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/ViBiOh/kmux/pkg/sha"
	"github.com/ViBiOh/kmux/pkg/table"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

var outputFormat string

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
			podChan := make(chan v1.Pod, 4)
			defer close(podChan)

			go watchTableOuput(kube, podChan)

			pods, err := kube.CoreV1().Pods(kube.Namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			alreadySeenPod := make(map[string]bool)
			sort.Sort(PodByAge(pods.Items))
			for _, pod := range pods.Items {
				podChan <- pod
				alreadySeenPod[shaPod(pod)] = true
			}

			watcher, err := resource.GetPodWatcher("namespace", kube.Namespace)(ctx, kube)
			if err != nil {
				return err
			}

			defer watcher.Stop()

			for event := range watcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				if alreadySeenPod[shaPod(*pod)] {
					continue
				}

				podChan <- *pod
			}

			return nil
		})
	},
}

// PodByAge sort v1.Pod by Age
type PodByAge []v1.Pod

func (a PodByAge) Len() int      { return len(a) }
func (a PodByAge) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PodByAge) Less(i, j int) bool {
	return a[i].Status.StartTime.Before(a[j].Status.StartTime)
}

func shaPod(pod v1.Pod) string {
	content, _ := json.Marshal(pod)
	return sha.New(content)
}

func initWatch() {
	flags := watchCmd.Flags()

	flags.StringVarP(&outputFormat, "output", "o", "", "Output format. One of: (wide)")
}

func watchTableOuput(kube client.Kube, pods <-chan v1.Pod) {
	defaultWidths := []uint64{
		45, 5, 8, 6, 14,
	}
	content := []table.Cell{
		table.NewCell("NAME"),
		table.NewCell("READY"),
		table.NewCell("PHASE"),
		table.NewCell("AGE"),
		table.NewCell("RESTARTS"),
	}

	isWide := outputFormat == "wide"

	if allNamespace {
		defaultWidths = append([]uint64{15}, defaultWidths...)
		content = append([]table.Cell{table.NewCell("NAMESPACE")}, content...)
	}

	if isWide {
		defaultWidths = append(defaultWidths, 12, 12, 14, 15)
		content = append(content,
			table.NewCell("IP"),
			table.NewCell("NODE"),
			table.NewCell("NOMINATED NODE"),
			table.NewCell("READINESS GATES"),
		)
	}

	watchTable := table.New(defaultWidths)
	kube.Std(watchTable.Format(content))

	for pod := range pods {
		content = content[:0]

		if allNamespace {
			content = append(content, table.NewCell(pod.Namespace))
		}

		phase, ready, restart, lastRestartDate := getPodStatus(pod)

		var since string
		if pod.Status.StartTime != nil {
			since = duration.HumanDuration(time.Since(pod.Status.StartTime.Time))
		}

		var restartText string
		if restart > 0 {
			restartText = fmt.Sprintf("%-14s", fmt.Sprintf("%d (%s ago)", restart, duration.HumanDuration(time.Since(lastRestartDate))))
		}

		var phaseCell table.Cell

		switch phase {
		case string(v1.PodRunning), string(v1.PodSucceeded):
			phaseCell = table.NewCellColor(phase, output.RawGreen)
		case string(v1.PodFailed):
			phaseCell = table.NewCellColor(phase, output.RawRed)
		case string(v1.PodPending), "ContainerCreating":
			phaseCell = table.NewCellColor(phase, output.RawCyan)
		case "Terminating":
			phaseCell = table.NewCellColor(phase, output.RawBlue)
		default:
			phaseCell = table.NewCellColor(phase, output.RawYellow)
		}

		var readyColor *color.Color
		total := len(pod.Status.ContainerStatuses)
		if ready != uint(total) {
			readyColor = output.RawYellow
		} else {
			readyColor = output.RawGreen
		}

		content = append(content,
			table.NewCell(pod.Name),
			table.NewCellColor(fmt.Sprintf("%d/%d", ready, total), readyColor),
			phaseCell,
			table.NewCell(since),
			table.NewCellColor(restartText, output.RawMagenta),
		)

		if isWide {
			ip, node, nominatedNode, readinessGates := getPodWide(pod)
			content = append(content,
				table.NewCell(ip),
				table.NewCell(node),
				table.NewCell(nominatedNode),
				table.NewCell(readinessGates),
			)
		}

		kube.Std(watchTable.Format(content))
	}
}

// from https://github.com/kubernetes/kubernetes/blob/v1.24.3/pkg/printers/internalversion/printers.go#L743
func getPodStatus(pod v1.Pod) (string, uint, uint, time.Time) {
	var ready uint
	var restart uint
	lastRestartDate := metav1.NewTime(time.Time{})

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false

	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]

		restart += uint(container.RestartCount)

		if container.LastTerminationState.Terminated != nil {
			terminatedDate := container.LastTerminationState.Terminated.FinishedAt
			if lastRestartDate.Before(&terminatedDate) {
				lastRestartDate = terminatedDate
			}
		}

		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue

		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true

		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true

		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}

	if !initializing {
		restart = 0
		hasRunning := false

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
				hasRunning = true
				ready++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			if hasPodReadyCondition(pod.Status.Conditions) {
				reason = "Running"
			} else {
				reason = "NotReady"
			}
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeUnreachablePodReason" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason, ready, restart, lastRestartDate.Time
}

func getPodWide(pod v1.Pod) (string, string, string, string) {
	nodeName := pod.Spec.NodeName
	nominatedNodeName := pod.Status.NominatedNodeName
	podIP := ""

	if len(pod.Status.PodIPs) > 0 {
		podIP = pod.Status.PodIPs[0].IP
	}

	if podIP == "" {
		podIP = "<none>"
	}
	if nodeName == "" {
		nodeName = "<none>"
	}
	if nominatedNodeName == "" {
		nominatedNodeName = "<none>"
	}

	readinessGates := "<none>"
	if len(pod.Spec.ReadinessGates) > 0 {
		trueConditions := 0
		for _, readinessGate := range pod.Spec.ReadinessGates {
			conditionType := readinessGate.ConditionType
			for _, condition := range pod.Status.Conditions {
				if condition.Type == conditionType {
					if condition.Status == v1.ConditionTrue {
						trueConditions++
					}
					break
				}
			}
		}
		readinessGates = fmt.Sprintf("%d/%d", trueConditions, len(pod.Spec.ReadinessGates))
	}

	return podIP, nodeName, nominatedNodeName, readinessGates
}

func hasPodReadyCondition(conditions []v1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
