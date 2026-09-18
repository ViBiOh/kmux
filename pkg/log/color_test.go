package log

import (
	"testing"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

var defaultKeys = []string{"level", "severity", "status", "statusCode"}

func TestColorOfJSON(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		content string
		keys    []string
		want    *color.Color
	}{
		"not json": {
			"a plain text log line",
			defaultKeys,
			output.White,
		},
		"no key wanted": {
			`{"level":"error"}`,
			nil,
			output.White,
		},
		"invalid json": {
			`{"level":`,
			defaultKeys,
			output.White,
		},
		"error level": {
			`{"level":"error"}`,
			defaultKeys,
			output.Red,
		},
		"level is case insensitive": {
			`{"LEVEL":"FATAL"}`,
			defaultKeys,
			output.Red,
		},
		"warning level": {
			`{"severity":"warning"}`,
			defaultKeys,
			output.Yellow,
		},
		"debug level": {
			`{"level":"debug"}`,
			defaultKeys,
			output.Green,
		},
		"info level": {
			`{"level":"info"}`,
			defaultKeys,
			output.White,
		},
		"unknown key": {
			`{"lvl":"error"}`,
			defaultKeys,
			output.White,
		},
		"server error status": {
			`{"status":503}`,
			defaultKeys,
			output.Red,
		},
		"client error status": {
			`{"status":404}`,
			defaultKeys,
			output.Yellow,
		},
		"redirect status": {
			`{"statusCode":301}`,
			defaultKeys,
			output.Green,
		},
		"ok status": {
			`{"status":200}`,
			defaultKeys,
			output.White,
		},
		"first key wins": {
			`{"level":"info","status":500}`,
			defaultKeys,
			output.White,
		},
		"key after another one": {
			`{"time":"now","message":"hello","level":"error"}`,
			defaultKeys,
			output.Red,
		},
		"nested object is skipped": {
			`{"http":{"level":"error"},"level":"debug"}`,
			defaultKeys,
			output.Green,
		},
		"nested array is skipped": {
			`{"tags":["level","error"],"level":"warn"}`,
			defaultKeys,
			output.Yellow,
		},
		"value equal to a key is not a key": {
			`{"message":"level","level":"error"}`,
			defaultKeys,
			output.Red,
		},
		"value equal to a key without any real key": {
			`{"message":"level"}`,
			defaultKeys,
			output.White,
		},
		"non string non number value": {
			`{"level":true}`,
			defaultKeys,
			output.White,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, ColorOfJSON(testCase.content, testCase.keys...))
		})
	}
}

func TestColorFromName(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		want *color.Color
	}{
		"red":     {"red", output.Red},
		"yellow":  {"yellow", output.Yellow},
		"white":   {"white", output.White},
		"green":   {"green", output.Green},
		"unknown": {"purple", nil},
		"empty":   {"", nil},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, ColorFromName(testCase.name))
		})
	}
}

func TestColorNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"red", "yellow", "white", "green"}, ColorNames())
}

func TestColorIsGreater(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		first  *color.Color
		second *color.Color
		want   bool
	}{
		"no color is filtered": {
			nil,
			output.Red,
			true,
		},
		"no filter keeps everything": {
			output.Green,
			nil,
			false,
		},
		"green is above red": {
			output.Green,
			output.Red,
			true,
		},
		"red is not above yellow": {
			output.Red,
			output.Yellow,
			false,
		},
		"same color is kept": {
			output.Yellow,
			output.Yellow,
			false,
		},
		"white is above yellow": {
			output.White,
			output.Yellow,
			true,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, colorIsGreater(testCase.first, testCase.second))
		})
	}
}
