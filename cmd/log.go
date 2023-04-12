package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/concurrent"
	"github.com/ViBiOh/kmux/pkg/log"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	dryRun    bool
	rawOutput bool

	since          time.Duration
	sinceSeconds   int64
	containers     []string
	labelsSelector map[string]string

	containersName   []string
	containersRegexp []*regexp.Regexp

	jsonColorKeys []string

	logFilter      string
	logColorFilter *color.Color
	logRegexp      *regexp.Regexp
)

var logCmd = &cobra.Command{
	Use:     "log TYPE NAME",
	Aliases: []string{"logs"},
	Short:   "Get logs of a given resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"cronjobs",
				"daemonsets",
				"deployments",
				"jobs",
				"namespaces",
				"nodes",
				"pods",
				"services",
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

			return getCommonObjects(cmd.Context(), viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 && len(labelsSelector) == 0 {
			return errors.New("either labels or `TYPE NAME` args must be specified")
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		sinceSeconds = int64(since.Seconds())

		for _, container := range containers {
			re, err := regexp.Compile(container)
			if err == nil {
				containersRegexp = append(containersRegexp, re)
			} else {
				containersName = append(containersName, container)
			}
		}

		if len(logFilter) > 0 {
			var err error

			logRegexp, err = regexp.Compile(logFilter)
			if err != nil {
				return fmt.Errorf("log filter compile: %w", err)
			}
		}

		if grepColor := viper.GetString("grepColor"); len(grepColor) != 0 {
			logColorFilter = log.ColorFromName(strings.ToLower(grepColor))
		}

		if levelKeys := viper.GetStringSlice("levelKeys"); len(levelKeys) != 0 {
			jsonColorKeys = append(jsonColorKeys, levelKeys...)
		}

		if statusCodeKeys := viper.GetStringSlice("statusCodeKeys"); len(statusCodeKeys) != 0 {
			jsonColorKeys = append(jsonColorKeys, statusCodeKeys...)
		}

		var resourceType, resourceName string
		if len(args) > 1 {
			resourceType = args[0]
			resourceName = args[1]
		}

		clients.Execute(ctx, func(ctx context.Context, kube client.Kube) error {
			podWatcher, err := resource.WatchPods(ctx, kube, resourceType, resourceName, labelsSelector, dryRun)
			if err != nil {
				return fmt.Errorf("watch pods: %w", err)
			}

			defer podWatcher.Stop()

			var activeStreams sync.Map

			streaming := concurrent.NewSimple()

			for event := range podWatcher.ResultChan() {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				streamCancel, ok := activeStreams.Load(pod.UID)

				if event.Type == watch.Deleted || event.Type == watch.Error || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
					if ok {
						streamCancel.(context.CancelFunc)()
						activeStreams.Delete(pod.UID)
					} else if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
						handleLogPod(ctx, &activeStreams, streaming, kube, *pod)
					}

					continue
				}

				if ok || pod.Status.Phase == v1.PodPending {
					continue
				}

				handleLogPod(ctx, &activeStreams, streaming, kube, *pod)
			}

			streaming.Wait()

			return nil
		})

		return nil
	},
}

func initLog() {
	flags := logCmd.Flags()

	flags.DurationVarP(&since, "since", "s", time.Hour, "Display logs since given duration")
	flags.StringSliceVarP(&containers, "containers", "c", nil, "Filter container's name, default to all containers, supports regexp")

	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
	flags.BoolVarP(&rawOutput, "raw-output", "r", false, "Raw ouput, don't print context or pod prefixes")

	flags.StringToStringVarP(&labelsSelector, "selector", "l", nil, "Labels to filter pods")

	flags.StringVarP(&logFilter, "grep", "g", "", "Regexp to filter log")

	flags.String("grepColor", "", "Get logs only above given color (red > yellow > green)")
	if err := viper.BindPFlag("grepColor", flags.Lookup("grepColor")); err != nil {
		output.Fatal("bind `grepColor` flag: %s", err)
	}

	flags.StringSlice("levelKeys", []string{"level", "severity"}, "Keys for level in JSON")
	if err := viper.BindPFlag("levelKeys", flags.Lookup("levelKeys")); err != nil {
		output.Fatal("bind `levelKeys` flag: %s", err)
	}

	flags.StringSlice("statusCodeKeys", []string{"status", "statusCode", "response_code", "http_status", "OriginStatus"}, "Keys for HTTP Status code in JSON")
	if err := viper.BindPFlag("statusCodeKeys", flags.Lookup("statusCodeKeys")); err != nil {
		output.Fatal("bind `statusCodeKeys` flag: %s", err)
	}
}

func handleLogPod(ctx context.Context, activeStreams *sync.Map, streaming *concurrent.Simple, kube client.Kube, pod v1.Pod) {
	for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		if !isContainerSelected(container) {
			continue
		}

		container := container

		if dryRun {
			kube.Info("%s %s", output.Green.Sprintf("[%s/%s]", pod.Name, container.Name), output.Yellow.Sprint("Found!"))
			continue
		}

		streaming.Go(func() {
			if pod.Status.Phase != v1.PodRunning {
				logPod(ctx, kube, pod.Namespace, pod.Name, container.Name)
				return
			}

			streamCtx, streamCancel := context.WithCancel(ctx)
			activeStreams.Store(pod.UID, streamCancel)
			defer streamCancel()

			streamPod(streamCtx, kube, pod.Namespace, pod.Name, container.Name)
		})
	}
}

func isContainerSelected(container v1.Container) bool {
	if len(containers) == 0 {
		return true
	}

	for _, containerRegexp := range containersRegexp {
		if containerRegexp.MatchString(container.Name) {
			return true
		}
	}

	for _, containerName := range containersName {
		if strings.EqualFold(containerName, container.Name) {
			return true
		}
	}

	return false
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
	outputter := kube.Child(rawOutput, output.Green.Sprintf("[%s/%s]", name, container))

	if !rawOutput {
		outputter.Info(output.Yellow.Sprint("Log..."))
		defer outputter.Info(output.Yellow.Sprint("Log ended."))
	}

	streamScanner := bufio.NewScanner(reader)
	streamScanner.Split(bufio.ScanLines)

	var colorOutputter *color.Color

	for streamScanner.Scan() {
		text := streamScanner.Text()

		colorOutputter = log.ColorOfJSON(text, jsonColorKeys...)

		if log.ColorIsGreater(colorOutputter, logColorFilter) {
			continue
		}

		if logRegexp == nil {
			outputter.Std(log.Format(text, colorOutputter))

			continue
		}

		if !logRegexp.MatchString(text) {
			continue
		}

		outputter.Std(log.FormatGrep(text, logRegexp, colorOutputter))
	}
}
