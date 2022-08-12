package table

import (
	"fmt"
	"strings"

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

		if cell.color != nil {
			cell.color.Fprintf(&builder, fmt.Sprintf("%%-%ds", width), cell.content)
		} else {
			fmt.Fprintf(&builder, fmt.Sprintf("%%-%ds", width), cell.content)
		}
	}

	return builder.String()
}

type Cell struct {
	content string
	color   *color.Color
}

func NewCell(content string) Cell {
	return Cell{
		content: content,
	}
}

func NewCellColor(content string, color *color.Color) Cell {
	return Cell{
		content: content,
		color:   color,
	}
}
