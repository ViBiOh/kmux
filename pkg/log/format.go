package log

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
)

func Format(text string, outputter *color.Color) string {
	if outputter == nil {
		return text
	}

	return outputter.Sprint(text)
}

func FormatGrep(text string, logFilters []*regexp.Regexp, outputter *color.Color) string {
	matches := mergeMatches(text, logFilters)
	if len(matches) == 0 {
		return Format(text, outputter)
	}

	highlight := output.Red
	if outputter == output.Red {
		highlight = output.Yellow
	}

	var greppedText strings.Builder
	var currentIndex int

	for _, match := range matches {
		if match[0] != currentIndex {
			greppedText.WriteString(Format(text[currentIndex:match[0]], outputter))
		}

		greppedText.WriteString(highlight.Sprint(text[match[0]:match[1]]))
		currentIndex = match[1]
	}

	if currentIndex != len(text) {
		greppedText.WriteString(Format(text[currentIndex:], outputter))
	}

	return greppedText.String()
}

func mergeMatches(text string, logFilters []*regexp.Regexp) [][]int {
	var matches [][]int

	for _, logFilter := range logFilters {
		matches = append(matches, logFilter.FindAllStringIndex(text, -1)...)
	}

	slices.SortFunc(matches, func(first, second []int) int {
		if first[0] != second[0] {
			return first[0] - second[0]
		}

		return second[1] - first[1]
	})

	merged := matches[:0]

	for _, match := range matches {
		if last := len(merged) - 1; last >= 0 && match[0] <= merged[last][1] {
			if match[1] > merged[last][1] {
				merged[last][1] = match[1]
			}

			continue
		}

		merged = append(merged, match)
	}

	return merged
}
