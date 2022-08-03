package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	blue = color.New(color.FgBlue).SprintFunc()
	red  = color.New(color.FgRed).SprintFunc()
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

func outputErr(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, red(fmt.Sprintf(format, args...)))
}

func outputErrAndExit(format string, args ...any) {
	outputErr("", format+"\n", args...)
	os.Exit(1)
}
