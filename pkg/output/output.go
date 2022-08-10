package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	Red    = color.New(color.FgRed).SprintFunc()
	Green  = color.New(color.FgGreen).SprintFunc()
	Blue   = color.New(color.FgBlue).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
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
	Err("", format+"\n", args...)
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
