package log

import (
	"regexp"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
)

type Formatter func(a ...any) string

func Format(text string, outputter *color.Color) string {
	if outputter == nil {
		return text
	}

	return outputter.Sprint(text)
}

func FormatGrep(text string, logFilter *regexp.Regexp, outputter *color.Color) string {
	var greppedText string
	var currentIndex int

	highlight := output.Red
	if outputter == output.Red {
		highlight = output.Yellow
	}

	for _, index := range logFilter.FindAllStringIndex(text, -1) {
		if index[0] != currentIndex {
			greppedText += Format(text[currentIndex:index[0]], outputter)
		}

		greppedText += highlight.Sprint(text[index[0]:index[1]])

		currentIndex = index[1]
	}

	if currentIndex != len(text) {
		greppedText += Format(text[currentIndex:], outputter)
	}

	return greppedText
}
