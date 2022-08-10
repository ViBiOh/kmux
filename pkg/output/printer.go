package output

import (
	"fmt"
	"os"
	"strings"
)

type event struct {
	std     bool
	prefix  string
	message string
}

var (
	done       = make(chan struct{})
	outputChan = make(chan event, 8)
)

func init() {
	go print()
}

func print() {
	defer close(done)

	for outputEvent := range outputChan {
		for _, line := range strings.Split(outputEvent.message, "\n") {
			if len(outputEvent.prefix) > 0 {
				fmt.Fprint(os.Stderr, outputEvent.prefix)
			}

			fd := os.Stderr
			if outputEvent.std {
				fd = os.Stdout
			}

			fmt.Fprint(fd, line, "\n")
		}
	}
}

func Close() {
	close(outputChan)
}

func Done() <-chan struct{} {
	return done
}

func outputContent(std bool, prefix, format string, args ...any) {
	outputChan <- event{std: std, prefix: prefix, message: fmt.Sprintf(format, args...)}
}
