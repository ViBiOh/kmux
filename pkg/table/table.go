package table

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
)

type Table struct {
	widths []uint64
	mutex  sync.Mutex
}

func New(defaultWidths []uint64) *Table {
	return &Table{
		widths: defaultWidths,
	}
}

// Format renders cells padded to the widest content seen so far. It is safe for
// concurrent use, one goroutine per cluster formats on the same table.
func (t *Table) Format(cells []Cell) string {
	var builder strings.Builder

	for i, cell := range cells {
		if i > 0 {
			builder.WriteString(" ")
		}

		width := t.widthFor(i, uint64(utf8.RuneCountInString(cell.content)))

		if _, err := cell.printer(&builder, fmt.Sprintf("%%-%ds", width), cell.content); err != nil {
			output.Err("", "printing table: %s", err)
		}
	}

	return builder.String()
}

func (t *Table) widthFor(index int, contentWidth uint64) uint64 {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if index >= len(t.widths) {
		t.widths = append(t.widths, contentWidth)

		return contentWidth
	}

	if contentWidth > t.widths[index] {
		t.widths[index] = contentWidth
	}

	return t.widths[index]
}

type Printer func(io.Writer, string, ...any) (int, error)

type Cell struct {
	printer Printer
	content string
}

func NewCell(content string) Cell {
	return Cell{
		content: content,
		printer: fmt.Fprintf,
	}
}

func NewCellColor(content string, color *color.Color) Cell {
	return Cell{
		content: content,
		printer: color.Fprintf,
	}
}
