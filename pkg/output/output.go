package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	RawRed     = color.New(color.FgRed)
	RawGreen   = color.New(color.FgGreen)
	RawBlue    = color.New(color.FgBlue)
	RawYellow  = color.New(color.FgYellow)
	RawCyan    = color.New(color.FgCyan)
	RawMagenta = color.New(color.FgMagenta)

	Red    = RawRed.SprintFunc()
	Green  = RawGreen.SprintFunc()
	Blue   = RawBlue.SprintFunc()
	Yellow = RawYellow.SprintFunc()
)

func Std(prefix, format string, args ...any) {
	outputContent(true, prefix, format, args...)
}

func Warn(prefix, format string, args ...any) {
	outputContent(false, prefix, Yellow(fmt.Sprintf(format, args...)))
}

func Err(prefix, format string, args ...any) {
	outputContent(false, prefix, Red(fmt.Sprintf(format, args...)))
}

func Info(prefix, format string, args ...any) {
	outputContent(false, prefix, fmt.Sprintf(format, args...))
}

func Fatal(format string, args ...any) {
	_, _ = fmt.Fprint(os.Stderr, Red(fmt.Sprintf(format, args...)))
	os.Exit(1)
}

type Outputter struct {
	prefix string
}

func NewOutputter(name string) Outputter {
	var prefix string

	if len(name) != 0 {
		prefix = Blue("[" + name + "] ")
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

func (o Outputter) Child(prefix string) Outputter {
	if len(prefix) != 0 {
		o.prefix += prefix + " "
	}

	return o
}
