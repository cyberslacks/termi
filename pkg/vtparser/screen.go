package vtparser

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Cell is one character position in the terminal framebuffer.
type Cell struct {
	Ch    rune
	Style lipgloss.Style
}

// Screen is a fixed-size VT100 framebuffer.
// All methods are NOT goroutine-safe — call from the Bubbletea Update() thread only.
type Screen struct {
	mu           sync.Mutex // only for external readers (scrollback copy)
	Cols, Rows   int
	cells        [][]Cell
	scrollback   [][]Cell // lines scrolled off the top
	maxScrollback int

	// Cursor
	curX, curY   int
	savedX, savedY int

	// Current SGR state
	curStyle lipgloss.Style

	// Parser state machine
	parser *parser
}

func NewScreen(cols, rows int) *Screen {
	s := &Screen{
		Cols:          cols,
		Rows:          rows,
		maxScrollback: 5000,
	}
	s.cells = makeGrid(cols, rows)
	s.parser = newParser(s)
	return s
}

func makeGrid(cols, rows int) [][]Cell {
	grid := make([][]Cell, rows)
	for i := range grid {
		grid[i] = makeRow(cols)
	}
	return grid
}

func makeRow(cols int) []Cell {
	row := make([]Cell, cols)
	for i := range row {
		row[i] = Cell{Ch: ' ', Style: lipgloss.NewStyle()}
	}
	return row
}

// Write feeds raw bytes (PTY output) into the parser state machine.
func (s *Screen) Write(data []byte) (int, error) {
	s.parser.feed(data)
	return len(data), nil
}

// Resize adapts the framebuffer to new dimensions.
func (s *Screen) Resize(cols, rows int) {
	if cols == s.Cols && rows == s.Rows {
		return
	}
	newCells := makeGrid(cols, rows)
	// Copy existing content that fits
	copyRows := min(s.Rows, rows)
	copyCols := min(s.Cols, cols)
	for r := 0; r < copyRows; r++ {
		for c := 0; c < copyCols; c++ {
			newCells[r][c] = s.cells[r][c]
		}
	}
	s.cells = newCells
	s.Cols = cols
	s.Rows = rows
	s.clampCursor()
}

// CellAt returns a copy of the cell at (row, col). Safe to call from render goroutine.
func (s *Screen) CellAt(row, col int) Cell {
	return s.cells[row][col]
}

// Cursor returns the current cursor position as (col, row).
func (s *Screen) Cursor() (col, row int) {
	return s.curX, s.curY
}

// RecentText returns the last n visible lines of the framebuffer as plain text,
// stripping SGR styling. Used to build AI context.
func (s *Screen) RecentText(n int) string {
	if n <= 0 {
		n = 50
	}
	start := s.Rows - n
	if start < 0 {
		start = 0
	}
	var sb []byte
	for r := start; r < s.Rows; r++ {
		for _, c := range s.cells[r] {
			if c.Ch == 0 {
				sb = append(sb, ' ')
			} else {
				sb = appendRune(sb, c.Ch)
			}
		}
		sb = append(sb, '\n')
	}
	return string(sb)
}

func appendRune(buf []byte, r rune) []byte {
	if r < 0x80 {
		return append(buf, byte(r))
	}
	var tmp [4]byte
	n := encodeRuneScreen(tmp[:], r)
	return append(buf, tmp[:n]...)
}

func encodeRuneScreen(p []byte, r rune) int {
	switch {
	case r < 0x80:
		p[0] = byte(r)
		return 1
	case r < 0x800:
		p[0] = byte(0xC0 | (r >> 6))
		p[1] = byte(0x80 | (r & 0x3F))
		return 2
	case r < 0x10000:
		p[0] = byte(0xE0 | (r >> 12))
		p[1] = byte(0x80 | ((r >> 6) & 0x3F))
		p[2] = byte(0x80 | (r & 0x3F))
		return 3
	default:
		p[0] = byte(0xF0 | (r >> 18))
		p[1] = byte(0x80 | ((r >> 12) & 0x3F))
		p[2] = byte(0x80 | ((r >> 6) & 0x3F))
		p[3] = byte(0x80 | (r & 0x3F))
		return 4
	}
}

// ScrollbackLines returns a copy of the scrollback buffer.
func (s *Screen) ScrollbackLines() [][]Cell {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]Cell, len(s.scrollback))
	copy(out, s.scrollback)
	return out
}

// --- internal helpers called by parser ---

func (s *Screen) putChar(ch rune) {
	if s.curX >= s.Cols {
		s.newline()
		s.curX = 0
	}
	s.cells[s.curY][s.curX] = Cell{Ch: ch, Style: s.curStyle}
	s.curX++
}

func (s *Screen) newline() {
	s.curY++
	if s.curY >= s.Rows {
		s.scrollUp()
		s.curY = s.Rows - 1
	}
}

func (s *Screen) scrollUp() {
	s.mu.Lock()
	s.scrollback = append(s.scrollback, s.cells[0])
	if len(s.scrollback) > s.maxScrollback {
		s.scrollback = s.scrollback[1:]
	}
	s.mu.Unlock()
	s.cells = append(s.cells[1:], makeRow(s.Cols))
}

func (s *Screen) clearLine(row, fromCol, toCol int) {
	for c := fromCol; c <= toCol && c < s.Cols; c++ {
		s.cells[row][c] = Cell{Ch: ' ', Style: lipgloss.NewStyle()}
	}
}

func (s *Screen) clearScreen(fromRow, fromCol, toRow, toCol int) {
	for r := fromRow; r <= toRow && r < s.Rows; r++ {
		startCol := 0
		endCol := s.Cols - 1
		if r == fromRow {
			startCol = fromCol
		}
		if r == toRow {
			endCol = toCol
		}
		s.clearLine(r, startCol, endCol)
	}
}

func (s *Screen) moveCursor(x, y int) {
	s.curX = x
	s.curY = y
	s.clampCursor()
}

func (s *Screen) clampCursor() {
	if s.curX < 0 {
		s.curX = 0
	}
	if s.curX >= s.Cols {
		s.curX = s.Cols - 1
	}
	if s.curY < 0 {
		s.curY = 0
	}
	if s.curY >= s.Rows {
		s.curY = s.Rows - 1
	}
}

func (s *Screen) saveCursor() {
	s.savedX, s.savedY = s.curX, s.curY
}

func (s *Screen) restoreCursor() {
	s.curX, s.curY = s.savedX, s.savedY
	s.clampCursor()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
