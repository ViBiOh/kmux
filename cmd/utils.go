package cmd

import (
	"fmt"
	"os"
	"strings"
)

func displayOutput(prefix, format string, args ...any) {
	if len(prefix) > 0 {
		prefix = "[" + prefix + "] "
	}

	for _, line := range strings.Split(fmt.Sprintf(format, args...), "\n") {
		fmt.Fprint(os.Stdout, prefix, line, "\n")
	}
}

func displayErrorAndExit(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
