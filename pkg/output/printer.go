package output

import (
	"fmt"
	"os"
	"strings"
)

type event struct {
	prefix  string
	message string
	std     bool
}

var (
	done       = make(chan struct{})
	outputChan = make(chan event, 8)
)

func init() {
	go startPrinter()
}

func startPrinter() {
	defer close(done)

	for outputEvent := range outputChan {
		message := strings.TrimSuffix(outputEvent.message, "\n")

		for line := range strings.SplitSeq(message, "\n") {
			if len(outputEvent.prefix) > 0 {
				_, _ = fmt.Fprint(os.Stderr, outputEvent.prefix)
			}

			fd := os.Stderr
			if outputEvent.std {
				fd = os.Stdout
			}

			_, _ = fmt.Fprint(fd, line, "\n")
		}
	}
}

func Close() {
	close(outputChan)
}

func Done() <-chan struct{} {
	return done
}

func outputContent(std bool, prefix, message string) {
	outputChan <- event{std: std, prefix: prefix, message: message}
}
