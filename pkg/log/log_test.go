package log

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLine(t *testing.T) {
	t.Parallel()

	type line struct {
		text      string
		truncated bool
	}

	cases := map[string]struct {
		input string
		limit int
		want  []line
	}{
		"empty input":               {"", 5, nil},
		"one line":                  {"abc\n", 5, []line{{"abc", false}}},
		"empty line":                {"\n", 5, []line{{"", false}}},
		"no trailing newline":       {"abc", 5, []line{{"abc", false}}},
		"carriage return":           {"abc\r\n", 5, []line{{"abc", false}}},
		"exactly the limit":         {"abcde\n", 5, []line{{"abcde", false}}},
		"crlf at the limit":         {"abcde\r\n", 5, []line{{"abcde", false}}},
		"crlf split across buffers": {strings.Repeat("a", 15) + "\r\nnext\n", 50, []line{{strings.Repeat("a", 15), false}, {"next", false}}},
		"inner carriage return":     {"ab\rcd\n", 5, []line{{"ab\rcd", false}}},
		"carriage return at eof":    {"abc\r", 5, []line{{"abc", false}}},
		"exactly the buffer size":   {strings.Repeat("a", 16) + "\nnext\n", 50, []line{{strings.Repeat("a", 16), false}, {"next", false}}},
		"cut inside a rune":         {"abcd\u00e9\n", 5, []line{{"abcd\xc3", true}}},
		"above the limit":           {"abcdefgh\nnext\n", 5, []line{{"abcde", true}, {"next", false}}},
		"above the limit at eof":    {"abcdefgh", 5, []line{{"abcde", true}}},
		"above the buffer size":     {strings.Repeat("a", 100) + "\nnext\n", 20, []line{{strings.Repeat("a", 20), true}, {"next", false}}},
		"above the buffer, in cap":  {strings.Repeat("a", 40) + "\nnext\n", 50, []line{{strings.Repeat("a", 40), false}, {"next", false}}},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			reader := bufio.NewReaderSize(strings.NewReader(testCase.input), 16)

			var got []line

			for {
				text, truncated, err := readLine(reader, testCase.limit)
				if err != nil {
					require.ErrorIs(t, err, io.EOF)

					break
				}

				got = append(got, line{text, truncated})
			}

			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestReadLineReadError(t *testing.T) {
	t.Parallel()

	errRead := errors.New("read failure")

	reader := bufio.NewReaderSize(io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(errRead)), 16)

	text, truncated, err := readLine(reader, 5)

	require.ErrorIs(t, err, errRead)
	assert.Empty(t, text)
	assert.False(t, truncated)
}
