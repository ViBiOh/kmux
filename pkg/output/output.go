package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	Red    = color.New(color.FgRed).SprintFunc()
	Green  = color.New(color.FgGreen).SprintFunc()
	Blue   = color.New(color.FgBlue).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
)

func outputContent(output io.Writer, prefix, format string, args ...any) {
	if len(prefix) > 0 {
		prefix = Blue("[" + prefix + "] ")
	}

	for _, line := range strings.Split(fmt.Sprintf(format, args...), "\n") {
		fmt.Fprint(output, prefix, line, "\n")
	}
}

func Std(prefix, format string, args ...any) {
	outputContent(os.Stdout, prefix, format, args...)
}

func Warn(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, Yellow(fmt.Sprintf(format, args...)))
}

func Err(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, Red(fmt.Sprintf(format, args...)))
}

func StdErr(prefix, format string, args ...any) {
	outputContent(os.Stderr, prefix, fmt.Sprintf(format, args...))
}

func Fatal(format string, args ...any) {
	Err("", format+"\n", args...)
	os.Exit(1)
}
