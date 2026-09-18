package log

import (
	"os"
	"regexp"
	"testing"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	color.NoColor = false

	os.Exit(m.Run())
}

func TestFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text      string
		outputter *color.Color
		want      string
	}{
		"no color": {
			"hello",
			nil,
			"hello",
		},
		"colored": {
			"hello",
			output.Red,
			output.Red.Sprint("hello"),
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, Format(testCase.text, testCase.outputter))
		})
	}
}

func TestFormatGrep(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text    string
		filters []string
		color   *color.Color
		want    string
	}{
		"no match keeps the line": {
			"hello world",
			[]string{"nope"},
			nil,
			"hello world",
		},
		"highlights the match": {
			"hello world",
			[]string{"world"},
			nil,
			"hello " + output.Red.Sprint("world"),
		},
		"highlights a match at the start": {
			"hello world",
			[]string{"hello"},
			nil,
			output.Red.Sprint("hello") + " world",
		},
		"highlights every match": {
			"a b a",
			[]string{"a"},
			nil,
			output.Red.Sprint("a") + " b " + output.Red.Sprint("a"),
		},
		"yellow highlight on a red line": {
			"hello world",
			[]string{"world"},
			output.Red,
			output.Red.Sprint("hello ") + output.Yellow.Sprint("world"),
		},
		"two filters are both highlighted": {
			"hello big world",
			[]string{"hello", "world"},
			nil,
			output.Red.Sprint("hello") + " big " + output.Red.Sprint("world"),
		},
		"overlapping filters are merged": {
			"hello world",
			[]string{"hello wo", "o world"},
			nil,
			output.Red.Sprint("hello world"),
		},
		"nested filters are merged": {
			"hello world",
			[]string{"hello world", "wor"},
			nil,
			output.Red.Sprint("hello world"),
		},
		"contiguous filters are merged": {
			"hello world",
			[]string{"hello ", "world"},
			nil,
			output.Red.Sprint("hello world"),
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			filters := make([]*regexp.Regexp, 0, len(testCase.filters))
			for _, filter := range testCase.filters {
				filters = append(filters, regexp.MustCompile(filter))
			}

			assert.Equal(t, testCase.want, FormatGrep(testCase.text, filters, testCase.color))
		})
	}
}

func TestFormatGrepDoesNotMatchEscapeCodes(t *testing.T) {
	t.Parallel()

	filters := []*regexp.Regexp{
		regexp.MustCompile("world"),
		regexp.MustCompile("[0-9;]+m"),
	}

	got := FormatGrep("hello world", filters, nil)

	assert.Equal(t, "hello "+output.Red.Sprint("world"), got)
}
