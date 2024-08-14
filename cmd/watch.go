package cmd

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
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

const noneValue = "<none>"

type watchPod struct {
	ContextName string `json:"context_name"`
	v1.Pod      `json:"pod"`
}

var (
	outputFormat    string
	showLabels      bool
	showAnnotations bool
	labelColumns    []string
)

func initWatch() {
	flags := watchCmd.Flags()

	flags.StringVarP(&outputFormat, "output", "o", "", "Output format. One of: (wide)")
	flags.StringToStringVarP(&labelsSelector, "selector", "l", nil, "Labels to filter pods")
	flags.BoolVarP(&showLabels, "show-labels", "", false, "Show all labels as the last column")
	flags.BoolVarP(&showAnnotations, "show-annotations", "", false, "Show all annotations as the last column (after labels if both asked)")
	flags.StringSliceVarP(&labelColumns, "label-columns", "L", nil, "Labels that are going to be presented as columns")
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Get all pods in the namespace",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		watchTable := initWatchTable()
		initialsPodsHash := displayInitialPods(ctx, watchTable)

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			watcher, err := resource.WatchPods(ctx, kube, "namespace", kube.Namespace, labelsSelector, false)
			if err != nil {
				return fmt.Errorf("watch pods: %w", err)
			}

			defer watcher.Stop()

			for event := range watcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				if initialsPodsHash[sha.JSON(*pod)] {
					continue
				}

				outputWatch(watchTable, kube.Name, *pod)
			}

			return nil
		})
	},
}

func initWatchTable() *table.Table {
	defaultWidths := []uint64{
		45, 5, 9, 6, 14,
	}
	content := []table.Cell{
		table.NewCell("NAME"),
		table.NewCell("READY"),
		table.NewCell("PHASE"),
		table.NewCell("AGE"),
		table.NewCell("RESTARTS"),
	}

	if allNamespace {
		defaultWidths = append([]uint64{15}, defaultWidths...)
		content = append([]table.Cell{table.NewCell("NAMESPACE")}, content...)
	}

	if len(clients) > 0 && len(clients[0].Name) != 0 {
		defaultWidths = append([]uint64{uint64(len(clients[0].Name))}, defaultWidths...)
		content = append([]table.Cell{table.NewCell("CONTEXT")}, content...)
	}

	if outputFormat == "wide" {
		defaultWidths = append(defaultWidths, 12, 12, 14, 15)
		content = append(content,
			table.NewCell("IP"),
			table.NewCell("NODE"),
			table.NewCell("NOMINATED NODE"),
			table.NewCell("READINESS GATES"),
		)
	}

	for _, label := range labelColumns {
		defaultWidths = append(defaultWidths, 12)
		content = append(content, table.NewCell(strings.ToUpper(path.Base(label))))
	}

	if showLabels {
		defaultWidths = append(defaultWidths, 12)
		content = append(content, table.NewCell("LABELS"))
	}

	if showAnnotations {
		defaultWidths = append(defaultWidths, 12)
		content = append(content, table.NewCell("ANNOTATIONS"))
	}

	watchTable := table.New(defaultWidths)
	output.Std("", "%s", watchTable.Format(content))

	return watchTable
}

// displayInitialPods for printing first list in chronological order
func displayInitialPods(ctx context.Context, watchTable *table.Table) map[string]bool {
	var listPods []watchPod
	initialPods := make(chan watchPod, 4)
	done := make(chan struct{})

	go func() {
		defer close(done)

		for pod := range initialPods {
			listPods = append(listPods, pod)
		}
	}()

	clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
		watcher, err := resource.WatchPods(ctx, kube, "namespace", kube.Namespace, labelsSelector, true)
		if err != nil {
			return fmt.Errorf("watch pods: %w", err)
		}

		defer watcher.Stop()

		for event := range watcher.ResultChan() {
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				continue
			}

			initialPods <- watchPod{
				Pod:         *pod,
				ContextName: kube.Name,
			}
		}

		return nil
	})

	close(initialPods)
	<-done

	sort.Sort(PodByAge(listPods))
	initialsPodsHash := make(map[string]bool)

	for _, pod := range listPods {
		initialsPodsHash[sha.JSON(pod.Pod)] = true
		outputWatch(watchTable, pod.ContextName, pod.Pod)
	}

	return initialsPodsHash
}

// PodByAge sort watchPod by Age.
type PodByAge []watchPod

func (a PodByAge) Len() int      { return len(a) }
func (a PodByAge) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PodByAge) Less(i, j int) bool {
	return a[i].Status.StartTime.Before(a[j].Status.StartTime)
}

func outputWatch(watchTable *table.Table, contextName string, pod v1.Pod) {
	var content []table.Cell

	if len(contextName) != 0 {
		content = append(content, table.NewCellColor(contextName, output.Blue))
	}

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
		restartValue := fmt.Sprintf("%d", restart)
		if !lastRestartDate.IsZero() {
			restartValue += fmt.Sprintf(" (%s ago)", duration.HumanDuration(time.Since(lastRestartDate)))
		}

		restartText = fmt.Sprintf("%-14s", restartValue)
	}

	var readyColor *color.Color
	total := len(pod.Status.ContainerStatuses)
	if ready != uint(total) {
		readyColor = output.Yellow
	} else {
		readyColor = output.Green
	}

	content = append(content,
		table.NewCell(pod.Name),
		table.NewCellColor(fmt.Sprintf("%d/%d", ready, total), readyColor),
		getPhaseCell(phase),
		table.NewCell(since),
		table.NewCellColor(restartText, output.Magenta),
	)

	if outputFormat == "wide" {
		ip, node, nominatedNode, readinessGates := getPodWide(pod)
		content = append(content,
			table.NewCell(ip),
			table.NewCell(node),
			table.NewCell(nominatedNode),
			table.NewCell(readinessGates),
		)
	}

	for _, label := range labelColumns {
		content = append(content, table.NewCell(pod.GetLabels()[label]))
	}

	if showLabels {
		content = append(content, table.NewCell(mapAsString(pod.GetLabels())))
	}

	if showAnnotations {
		content = append(content, table.NewCell(mapAsString(pod.GetAnnotations())))
	}

	output.Std("", "%s", watchTable.Format(content))
}

func getPhaseCell(phase string) table.Cell {
	switch phase {
	case string(v1.PodRunning), string(v1.PodSucceeded), "Completed":
		return table.NewCellColor(phase, output.Green)
	case string(v1.PodFailed), "CrashLoopBackOff", "ImagePullBackOff", "Error":
		return table.NewCellColor(phase, output.Red)
	case string(v1.PodPending), "ContainerCreating":
		return table.NewCellColor(phase, output.Cyan)
	case "Terminated", "Terminating":
		return table.NewCellColor(phase, output.Blue)
	default:
		return table.NewCellColor(phase, output.Yellow)
	}
}

func mapAsString(labels map[string]string) string {
	values := make([]string, len(labels))

	var index int
	for key, value := range labels {
		values[index] = fmt.Sprintf("%s=%s", key, value)
		index++
	}

	sort.Strings(values)

	return strings.Join(values, ",")
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
		var exitCode int32

		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restart += uint(container.RestartCount)

			if container.LastTerminationState.Terminated != nil {
				terminatedDate := container.LastTerminationState.Terminated.FinishedAt
				if lastRestartDate.Before(&terminatedDate) {
					lastRestartDate = terminatedDate
				}
			}

			var containerReason string
			var containerExitCode int32

			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				containerReason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				containerReason = container.State.Terminated.Reason
				containerExitCode = container.State.Terminated.ExitCode
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					containerReason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					containerReason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				ready++
			}

			if exitCode == 0 && len(containerReason) > 0 {
				reason = containerReason
				exitCode = containerExitCode
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
		podIP = noneValue
	}
	if nodeName == "" {
		nodeName = noneValue
	}
	if nominatedNodeName == "" {
		nominatedNodeName = noneValue
	}

	readinessGates := noneValue
	if len(pod.Spec.ReadinessGates) > 0 {
		readinessGates = fmt.Sprintf("%d/%d", getTrueReadyConditions(pod.Spec.ReadinessGates, pod.Status.Conditions), len(pod.Spec.ReadinessGates))
	}

	return podIP, nodeName, nominatedNodeName, readinessGates
}

func getTrueReadyConditions(gates []v1.PodReadinessGate, conditions []v1.PodCondition) uint {
	var ready uint

	for _, readinessGate := range gates {
		conditionType := readinessGate.ConditionType

		for _, condition := range conditions {
			if condition.Type == conditionType {
				if condition.Status == v1.ConditionTrue {
					ready++
				}
				break
			}
		}
	}

	return ready
}

func hasPodReadyCondition(conditions []v1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
