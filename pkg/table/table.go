package table

import (
	"fmt"
	"io"
	"strings"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
)

type Table struct {
	widths []uint64
}

func New(defaultWidths []uint64) *Table {
	return &Table{
		widths: defaultWidths,
	}
}

func (t *Table) Format(cells []Cell) string {
	var builder strings.Builder

	for i, cell := range cells {
		if i > 0 {
			builder.WriteString(" ")
		}

		var width uint64

		contentWidth := uint64(len(cell.content))

		if i >= len(t.widths) {
			t.widths = append(t.widths, contentWidth)
			width = contentWidth
		} else if width = t.widths[i]; contentWidth > width {
			t.widths[i] = contentWidth
			width = contentWidth
		}

		if _, err := cell.printer(&builder, fmt.Sprintf("%%-%ds", width), cell.content); err != nil {
			output.Err("", "printing table: %s", err)
		}
	}

	return builder.String()
}

type Printer func(io.Writer, string, ...any) (int, error)

type Cell struct {
	content string
	printer Printer
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
