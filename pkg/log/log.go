package log

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/fatih/color"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	maxLineSize = 1024 * 1024
	readerSize  = 64 * 1024
)

type Logger struct {
	selector        map[string]string
	logRegexes      []*regexp.Regexp
	containerRegexp *regexp.Regexp
	colorFilter     *color.Color
	kind            string
	name            string
	jsonColorKeys   []string
	since           int64
	rawOutput       bool
	dryRun          bool
	invertRegexp    bool
	noFollow        bool
}

func NewLogger(kind, name string, selector map[string]string, since time.Duration) Logger {
	return Logger{
		kind:     kind,
		name:     name,
		selector: selector,
		since:    int64(since.Seconds()),
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
	podWatcher, err := resource.WatchPods(ctx, kube, l.kind, l.name, l.selector, l.dryRun || l.noFollow)
	if err != nil {
		return fmt.Errorf("watch pods: %w", err)
	}

	defer podWatcher.Stop()

	streams := newStreamSet()

	var streaming sync.WaitGroup

	for event := range podWatcher.ResultChan() {
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}

		isTerminated := pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed
		isGone := event.Type == watch.Deleted || event.Type == watch.Error

		if isGone || isTerminated {
			if !streams.markTerminal(pod.UID) {
				continue
			}

			if streams.cancelPod(pod.UID) || !isTerminated {
				continue
			}

			l.handlePod(ctx, kube, streams, &streaming, *pod)

			continue
		}

		if pod.Status.Phase == v1.PodPending || streams.isTerminal(pod.UID) {
			continue
		}

		l.handlePod(ctx, kube, streams, &streaming, *pod)
	}

	streaming.Wait()

	return nil
}

func (l Logger) handlePod(ctx context.Context, kube client.Kube, streams *streamSet, streaming *sync.WaitGroup, pod v1.Pod) {
	for _, container := range resource.SelectedContainers(pod.Spec, l.containerRegexp) {
		if l.dryRun {
			kube.Info("%s %s", output.Green.Sprintf("[%s/%s]", pod.Name, container.Name), output.Yellow.Sprint("Found!"))

			continue
		}

		if pod.Status.Phase != v1.PodRunning {
			streaming.Go(func() {
				l.streamPod(ctx, kube, pod.Namespace, pod.Name, container.Name, false)
			})

			continue
		}

		streamCtx, ok := streams.add(ctx, pod.UID, container.Name)
		if !ok {
			continue
		}

		streaming.Go(func() {
			defer streams.remove(pod.UID, container.Name)

			l.streamPod(streamCtx, kube, pod.Namespace, pod.Name, container.Name, !l.noFollow)
		})
	}
}

func (l Logger) streamPod(ctx context.Context, kube client.Kube, namespace, name, container string, follow bool) {
	stream, err := kube.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Follow:       follow,
		SinceSeconds: l.sinceSeconds(),
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

func (l Logger) sinceSeconds() *int64 {
	if l.since <= 0 {
		return nil
	}

	return &l.since
}

func (l Logger) logOutputter(kube client.Kube, name, container string) output.Outputter {
	return kube.Child(l.rawOutput, output.Green.Sprintf("[%s/%s]", name, container))
}

func (l Logger) outputLog(reader io.Reader, outputter output.Outputter) {
	if !l.rawOutput {
		outputter.Warn("Log...")
		defer outputter.Warn("Log ended.")
	}

	lineReader := bufio.NewReaderSize(reader, readerSize)

	for {
		line, truncated, err := readLine(lineReader, maxLineSize)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
				outputter.Err("read logs: %s", err)
			}

			return
		}

		l.outputLine(line, outputter)

		if truncated {
			outputter.Err("log line above %d bytes, output truncated", maxLineSize)
		}
	}
}

// readLine keeps the first limit bytes of a line and discards the rest, so an oversized line does not end the stream.
func readLine(reader *bufio.Reader, limit int) (string, bool, error) {
	var line []byte

	var truncated, pendingCR bool

	for {
		chunk, err := reader.ReadSlice('\n')
		isLast := err == nil || errors.Is(err, io.EOF)

		if err == nil {
			chunk = chunk[:len(chunk)-1]
		}

		// A carriage return ending a chunk is only dropped when it ends the line, the next chunk tells.
		if pendingCR && len(chunk) > 0 {
			line, truncated = appendCapped(line, []byte{'\r'}, limit, truncated)
			pendingCR = false
		}

		if bytes.HasSuffix(chunk, []byte{'\r'}) {
			chunk = chunk[:len(chunk)-1]
			pendingCR = true
		}

		line, truncated = appendCapped(line, chunk, limit, truncated)

		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}

		if err != nil && (!isLast || len(line) == 0 && !truncated) {
			return "", false, err
		}

		return string(line), truncated, nil
	}
}

func appendCapped(line, chunk []byte, limit int, truncated bool) ([]byte, bool) {
	if room := limit - len(line); len(chunk) > room {
		return append(line, chunk[:room]...), true
	}

	return append(line, chunk...), truncated
}

func (l Logger) outputLine(text string, outputter output.Outputter) {
	text = output.Sanitize(text)

	lineColor := ColorOfJSON(text, l.jsonColorKeys...)

	if colorIsGreater(lineColor, l.colorFilter) {
		return
	}

	if len(l.logRegexes) == 0 {
		outputter.Std("%s", Format(text, lineColor))

		return
	}

	if !l.grepMatch(text) {
		return
	}

	outputter.Std("%s", FormatGrep(text, l.logRegexes, lineColor))
}

func (l Logger) grepMatch(text string) bool {
	for _, logRegexp := range l.logRegexes {
		if logRegexp.MatchString(text) {
			return !l.invertRegexp
		}
	}

	return l.invertRegexp
}
