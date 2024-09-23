package log

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/concurrent"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/fatih/color"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type Logger struct {
	selector        map[string]string
	logRegexes      []*regexp.Regexp
	containerRegexp *regexp.Regexp
	colorFilter     *color.Color
	resourceType    string
	resourceName    string
	jsonColorKeys   []string
	since           int64
	rawOutput       bool
	dryRun          bool
	invertRegexp    bool
	noFollow        bool
}

func NewLogger(resourceType, resourceName string, selector map[string]string, since time.Duration) Logger {
	return Logger{
		resourceType: resourceType,
		resourceName: resourceName,
		selector:     selector,
		since:        int64(since.Seconds()),
	}
}

func (l Logger) WithDryRun(dryRun bool) Logger {
	l.dryRun = dryRun

	return l
}

func (l Logger) WithContainerRegexp(containerRegexp *regexp.Regexp) Logger {
	l.containerRegexp = containerRegexp

	return l
}

func (l Logger) WithNoFollow(noFollow bool) Logger {
	l.noFollow = noFollow

	return l
}

func (l Logger) WithLogRegexes(logRegexes []*regexp.Regexp) Logger {
	l.logRegexes = logRegexes

	return l
}

func (l Logger) WithInvertRegexp(invertRegexp bool) Logger {
	l.invertRegexp = invertRegexp

	return l
}

func (l Logger) WithColorFilter(colorFilter *color.Color) Logger {
	l.colorFilter = colorFilter

	return l
}

func (l Logger) WithJsonColorKeys(jsonColorKeys []string) Logger {
	l.jsonColorKeys = jsonColorKeys

	return l
}

func (l Logger) WithRawOutput(rawOutput bool) Logger {
	l.rawOutput = rawOutput

	return l
}

func (l Logger) Log(ctx context.Context, kube client.Kube) error {
	podWatcher, err := resource.WatchPods(ctx, kube, l.resourceType, l.resourceName, l.selector, l.dryRun || l.noFollow)
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

				if event.Type == watch.Deleted {
					activeStreams.Delete(pod.UID)
				}
			} else if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
				l.handlePod(ctx, kube, &activeStreams, streaming, *pod)
			}

			continue
		}

		if ok || pod.Status.Phase == v1.PodPending {
			continue
		}

		l.handlePod(ctx, kube, &activeStreams, streaming, *pod)
	}

	streaming.Wait()

	return nil
}

func (l Logger) handlePod(ctx context.Context, kube client.Kube, activeStreams *sync.Map, streaming *concurrent.Simple, pod v1.Pod) {
	for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		if !resource.IsContainedSelected(container, l.containerRegexp) {
			continue
		}

		container := container

		if l.dryRun {
			kube.Info("%s %s", output.Green.Sprintf("[%s/%s]", pod.Name, container.Name), output.Yellow.Sprint("Found!"))
			continue
		}

		streaming.Go(func() {
			if pod.Status.Phase != v1.PodRunning {
				l.logPod(ctx, kube, pod.Namespace, pod.Name, container.Name)
				return
			}

			streamCtx, streamCancel := context.WithCancel(ctx)
			activeStreams.Store(pod.UID, streamCancel)
			defer streamCancel()

			l.streamPod(streamCtx, kube, pod.Namespace, pod.Name, container.Name)
		})
	}
}

func (l Logger) logPod(ctx context.Context, kube client.Kube, namespace, name, container string) {
	content, err := kube.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		SinceSeconds: &l.since,
		Container:    container,
	}).DoRaw(ctx)
	if err != nil {
		kube.Err("get logs: %s", err)
		return
	}

	l.outputLog(bytes.NewReader(content), l.logOutputter(kube, name, container))
}

func (l Logger) streamPod(ctx context.Context, kube client.Kube, namespace, name, container string) {
	stream, err := kube.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Follow:       !l.noFollow,
		SinceSeconds: &l.since,
		Container:    container,
	}).Stream(ctx)
	if err != nil {
		kube.Err("stream logs: %s", err)
		return
	}

	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			kube.Err("close stream: %s", closeErr)
		}
	}()

	l.outputLog(stream, l.logOutputter(kube, name, container))
}

func (l Logger) logOutputter(kube client.Kube, name, container string) output.Outputter {
	return kube.Child(l.rawOutput, output.Green.Sprintf("[%s/%s]", name, container))
}

func (l Logger) outputLog(reader io.Reader, outputter output.Outputter) {
	if !l.rawOutput {
		outputter.Warn("Log...")
		defer outputter.Warn("Log ended.")
	}

	streamScanner := bufio.NewScanner(reader)
	streamScanner.Split(bufio.ScanLines)

	var colorOutputter *color.Color

	for streamScanner.Scan() {
		text := streamScanner.Text()

		colorOutputter = ColorOfJSON(text, l.jsonColorKeys...)

		if colorIsGreater(colorOutputter, l.colorFilter) {
			continue
		}

		if len(l.logRegexes) == 0 {
			outputter.Std("%s", Format(text, colorOutputter))

			continue
		}

		if !l.grepMatch(text) {
			continue
		}

		greppedText := text
		for _, logRegexp := range l.logRegexes {
			greppedText = FormatGrep(greppedText, logRegexp, colorOutputter)
		}

		outputter.Std("%s", greppedText)
	}
}

func (l Logger) grepMatch(text string) bool {
	for _, logRegexp := range l.logRegexes {
		if logRegexp.MatchString(text) {
			return !l.invertRegexp
		}
	}

	return false
}
