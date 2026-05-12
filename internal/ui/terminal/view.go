package terminal

import (
	"strings"

	"github.com/jtfrow/termi/pkg/vtparser"
)

// View renders the VT framebuffer cell-by-cell using lipgloss styles.
func (m Model) View() string {
	if m.screen == nil {
		return ""
	}

	if m.scrollOffset > 0 {
		return m.renderScrollback()
	}

	var sb strings.Builder
	sb.Grow(m.width * m.height * 4)

	for row := 0; row < m.screen.Rows; row++ {
		for col := 0; col < m.screen.Cols; col++ {
			cell := m.screen.CellAt(row, col)
			sb.WriteString(renderCell(cell))
		}
		if row < m.screen.Rows-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func (m Model) renderScrollback() string {
	scrollback := m.screen.ScrollbackLines()
	total := len(scrollback)
	offset := m.scrollOffset
	if offset > total {
		offset = total
	}
	startLine := total - offset
	if startLine < 0 {
		startLine = 0
	}

	lines := scrollback[startLine:]
	// Pad with screen content if needed
	screenRows := m.height - len(lines)
	if screenRows < 0 {
		screenRows = 0
		lines = lines[len(lines)-m.height:]
	}

	var sb strings.Builder
	for _, row := range lines {
		for _, cell := range row {
			sb.WriteString(renderCell(cell))
		}
		sb.WriteByte('\n')
	}
	for row := 0; row < screenRows; row++ {
		for col := 0; col < m.screen.Cols; col++ {
			cell := m.screen.CellAt(row, col)
			sb.WriteString(renderCell(cell))
		}
		if row < screenRows-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func renderCell(cell vtparser.Cell) string {
	ch := cell.Ch
	if ch == 0 {
		ch = ' '
	}
	return cell.Style.Render(string(ch))
}
