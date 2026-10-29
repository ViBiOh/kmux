package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	Blue    = color.New(color.FgBlue)
	Cyan    = color.New(color.FgCyan)
	Green   = color.New(color.FgGreen)
	Magenta = color.New(color.FgMagenta)
	Red     = color.New(color.FgRed)
	White   = color.New(color.FgWhite)
	Yellow  = color.New(color.FgYellow)
)

func Std(prefix, format string, args ...any) {
	outputContent(true, prefix, fmt.Sprintf(format, args...))
}

func Warn(prefix, format string, args ...any) {
	outputContent(false, prefix, Yellow.Sprint(Sanitize(fmt.Sprintf(format, args...))))
}

func Err(prefix, format string, args ...any) {
	outputContent(false, prefix, Red.Sprint(Sanitize(fmt.Sprintf(format, args...))))
}

func Info(prefix, format string, args ...any) {
	outputContent(false, prefix, fmt.Sprintf(format, args...))
}

// Sanitize drops control characters, other than newline and tab, from content
// coming from the cluster, so it cannot drive the terminal with escape sequences.
// Line separators and bidirectional overrides are dropped too, they spoof the output.
func Sanitize(content string) string {
	return strings.Map(func(char rune) rune {
		switch {
		case char == '\n', char == '\t':
			return char
		case char < 0x20, char >= 0x7f && char <= 0x9f,
			char == 0x2028, char == 0x2029,
			char >= 0x202a && char <= 0x202e,
			char >= 0x2066 && char <= 0x2069:
			return -1
		default:
			return char
		}
	}, content)
}

func Fatal(format string, args ...any) {
	Close()
	<-Done()

	_, _ = fmt.Fprintln(os.Stderr, Red.Sprint(Sanitize(fmt.Sprintf(format, args...))))

	os.Exit(1)
}

type Outputter struct {
	prefix string
}

func NewOutputter(name string) Outputter {
	var prefix string

	if len(name) != 0 {
		prefix = Blue.Sprint("[" + name + "] ")
	}

	return Outputter{
		prefix: prefix,
	}
}

func (o Outputter) Write(payload []byte) (int, error) {
	Std(o.prefix, "%s", payload)
	return len(payload), nil
}

func (o Outputter) Std(format string, args ...any) {
	Std(o.prefix, format, args...)
}

func (o Outputter) Err(format string, args ...any) {
	Err(o.prefix, format, args...)
}

func (o Outputter) Warn(format string, args ...any) {
	Warn(o.prefix, format, args...)
}

func (o Outputter) Info(format string, args ...any) {
	Info(o.prefix, format, args...)
}

func (o Outputter) Child(noPrefix bool, prefix string) Outputter {
	if noPrefix {
		o.prefix = ""
	} else if len(prefix) != 0 {
		o.prefix += prefix + " "
	}

	return o
}
