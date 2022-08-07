package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/fatih/color"
)

var (
	red    = color.New(color.FgRed).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	blue   = color.New(color.FgBlue).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
)

func outputContent(output io.Writer, prefix, format string, args ...any) {
	if len(prefix) > 0 {
		prefix = blue("[" + prefix + "] ")
	}

	for _, line := range strings.Split(fmt.Sprintf(format, args...), "\n") {
		fmt.Fprint(output, prefix, line, "\n")
	}
}

func outputStd(prefix, format string, args ...any) {
	outputContent(os.Stdout, prefix, format, args...)
}

func outputWarn(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, yellow(fmt.Sprintf(format, args...)))
}

func outputErr(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, red(fmt.Sprintf(format, args...)))
}

func outputStdErr(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, fmt.Sprintf(format, args...))
}

func outputErrAndExit(format string, args ...any) {
	outputErr("", format+"\n", args...)
	os.Exit(1)
}

func waitForEnd(signals ...os.Signal) {
	signalsChan := make(chan os.Signal, len(signals))
	defer close(signalsChan)

	signal.Notify(signalsChan, signals...)
	defer signal.Stop(signalsChan)

	sig := <-signalsChan
	outputWarn("", "Signal %s received!", sig)
}
