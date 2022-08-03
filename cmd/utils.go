package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	blue = color.New(color.FgBlue).SprintFunc()
	red  = color.New(color.FgRed).SprintFunc()
)

func displayOutput(prefix, format string, args ...any) {
	if len(prefix) > 0 {
		prefix = blue("[" + prefix + "] ")
	}

	for _, line := range strings.Split(fmt.Sprintf(format, args...), "\n") {
		fmt.Fprint(os.Stdout, prefix, line, "\n")
	}
}

func displayErrorAndExit(format string, args ...any) {
	fmt.Fprint(os.Stderr, red(fmt.Sprintf(format+"\n", args...)))
	os.Exit(1)
}
