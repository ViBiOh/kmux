package output

import (
	"fmt"
	"os"

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
	outputContent(false, prefix, Yellow.Sprintf(format, args...))
}

func Err(prefix, format string, args ...any) {
	outputContent(false, prefix, Red.Sprintf(format, args...))
}

func Info(prefix, format string, args ...any) {
	outputContent(false, prefix, fmt.Sprintf(format, args...))
}

func Fatal(format string, args ...any) {
	_, _ = fmt.Fprint(os.Stderr, Red.Sprintf(format, args...))
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
