package output

import (
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
)

type event struct {
	prefix  string
	message string
	std     bool
}

type printer struct {
	stdout io.Writer
	stderr io.Writer
	events chan event
	done   chan struct{}
	mutex  sync.RWMutex
	closed bool
}

var defaultPrinter = newPrinter(os.Stdout, os.Stderr)

func init() {
	go defaultPrinter.start()
}

func newPrinter(stdout, stderr io.Writer) *printer {
	return &printer{
		stdout: stdout,
		stderr: stderr,
		events: make(chan event, 128),
		done:   make(chan struct{}),
	}
}

// start consumes events until close is called. Writes are buffered, a buffer is
// flushed before writing on the other one so the relative order of stdout and
// stderr is kept when both are attached to the same terminal.
func (p *printer) start() {
	defer close(p.done)

	stdout := bufio.NewWriter(p.stdout)
	stderr := bufio.NewWriter(p.stderr)

	defer flush(stdout, stderr)

	for outputEvent := range p.events {
		message := strings.TrimSuffix(outputEvent.message, "\n")

		for line := range strings.SplitSeq(message, "\n") {
			if len(outputEvent.prefix) > 0 {
				write(stderr, stdout, outputEvent.prefix)
			}

			if outputEvent.std {
				write(stdout, stderr, line+"\n")
			} else {
				write(stderr, stdout, line+"\n")
			}
		}

		if len(p.events) == 0 {
			flush(stdout, stderr)
		}
	}
}

func (p *printer) send(outputEvent event) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.closed {
		return
	}

	p.events <- outputEvent
}

func (p *printer) close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	close(p.events)
}

func write(target, other *bufio.Writer, content string) {
	if other.Buffered() > 0 {
		_ = other.Flush()
	}

	_, _ = target.WriteString(content)
}

func flush(writers ...*bufio.Writer) {
	for _, writer := range writers {
		_ = writer.Flush()
	}
}

// Close stops the printer, it is safe to call it more than once and any output
// sent afterwards is dropped instead of panicking.
func Close() {
	defaultPrinter.close()
}

func Done() <-chan struct{} {
	return defaultPrinter.done
}

func outputContent(std bool, prefix, message string) {
	defaultPrinter.send(event{std: std, prefix: prefix, message: message})
}
