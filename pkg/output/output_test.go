package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitize(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input string
		want  string
	}{
		"plain text":                {"hello world", "hello world"},
		"keeps newline and tab":     {"a\tb\nc", "a\tb\nc"},
		"keeps unicode":             {"héllo 日本", "héllo 日本"},
		"drops escape":              {"\x1b[31mred\x1b[0m", "[31mred[0m"},
		"drops osc title":           {"\x1b]0;pwned\x07text", "]0;pwnedtext"},
		"drops carriage return":     {"safe\rfake", "safefake"},
		"drops backspace and bell":  {"a\x08b\x07", "ab"},
		"drops delete":              {"a\x7fb", "ab"},
		"drops c1 control":          {"a\u009bb", "ab"},
		"drops bidi override":       {"a\u202eb", "ab"},
		"drops line separator":      {"a\u2028b", "ab"},
		"keeps non control latin 1": {" é", " é"},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, Sanitize(testCase.input))
		})
	}
}
