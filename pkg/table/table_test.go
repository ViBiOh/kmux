package table

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		defaultWidths []uint64
		rows          [][]string
		want          []string
	}{
		"pads to the default width": {
			[]uint64{5},
			[][]string{{"ab"}},
			[]string{"ab   "},
		},
		"grows with the content": {
			[]uint64{2},
			[][]string{{"abcd"}, {"ab"}},
			[]string{"abcd", "ab  "},
		},
		"appends unknown columns": {
			[]uint64{2},
			[][]string{{"ab", "cdef"}},
			[]string{"ab cdef"},
		},
		"separates cells with a space": {
			[]uint64{3, 3},
			[][]string{{"a", "b"}},
			[]string{"a   b  "},
		},
		"counts runes and not bytes": {
			[]uint64{0},
			[][]string{{"héllo"}, {"ab"}},
			[]string{"héllo", "ab   "},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			instance := New(testCase.defaultWidths)

			got := make([]string, 0, len(testCase.rows))

			for _, row := range testCase.rows {
				cells := make([]Cell, 0, len(row))
				for _, content := range row {
					cells = append(cells, NewCell(content))
				}

				got = append(got, instance.Format(cells))
			}

			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestFormatConcurrently(t *testing.T) {
	t.Parallel()

	instance := New([]uint64{5, 5})

	var wg sync.WaitGroup

	for index := range 16 {
		wg.Go(func() {
			for range 16 {
				assert.NotEmpty(t, instance.Format([]Cell{
					NewCell("pod"),
					NewCell("namespace"),
					NewCell(string(rune('a' + index))),
				}))
			}
		})
	}

	wg.Wait()

	assert.Equal(t, []uint64{5, 9, 1}, instance.widths)
}
