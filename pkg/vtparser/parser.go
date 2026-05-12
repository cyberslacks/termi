package vtparser

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// parser wraps charmbracelet/x/ansi and translates events into Screen mutations.
type parser struct {
	screen  *Screen
	p       *ansi.Parser
	handler ansi.Handler
}

func newParser(s *Screen) *parser {
	p := &parser{screen: s}
	p.p = ansi.NewParser()
	p.handler = ansi.Handler{
		Print:     p.handlePrint,
		Execute:   p.handleExecute,
		HandleCsi: p.handleCSI,
		HandleEsc: p.handleESC,
	}
	p.p.SetHandler(p.handler)
	return p
}

func (p *parser) feed(data []byte) {
	p.p.Parse(data)
}

func (p *parser) handlePrint(r rune) {
	p.screen.putChar(r)
}

func (p *parser) handleExecute(b byte) {
	s := p.screen
	switch b {
	case '\r':
		s.curX = 0
	case '\n':
		s.newline()
	case '\b':
		if s.curX > 0 {
			s.curX--
		}
	case '\t':
		next := (s.curX/8 + 1) * 8
		if next >= s.Cols {
			next = s.Cols - 1
		}
		for s.curX < next {
			s.putChar(' ')
		}
	case 0x07: // BEL — ignore
	}
}

func (p *parser) handleESC(cmd ansi.Cmd) {
	s := p.screen
	switch cmd.Final() {
	case '7': // DECSC save cursor
		s.saveCursor()
	case '8': // DECRC restore cursor
		s.restoreCursor()
	case 'M': // RI reverse index
		if s.curY == 0 {
			s.cells = append([][]Cell{makeRow(s.Cols)}, s.cells[:s.Rows-1]...)
		} else {
			s.curY--
		}
	}
}

func (p *parser) handleCSI(cmd ansi.Cmd, params ansi.Params) {
	s := p.screen

	// Helper: get param at index with default.
	param := func(idx, def int) int {
		v, _, _ := params.Param(idx, def)
		return v
	}

	switch cmd.Final() {
	case 'A': // CUU cursor up
		s.curY -= param(0, 1)
		s.clampCursor()
	case 'B': // CUD cursor down
		s.curY += param(0, 1)
		s.clampCursor()
	case 'C': // CUF cursor forward
		s.curX += param(0, 1)
		s.clampCursor()
	case 'D': // CUB cursor back
		s.curX -= param(0, 1)
		s.clampCursor()
	case 'E': // CNL
		s.curY += param(0, 1)
		s.curX = 0
		s.clampCursor()
	case 'F': // CPL
		s.curY -= param(0, 1)
		s.curX = 0
		s.clampCursor()
	case 'G': // CHA
		s.curX = param(0, 1) - 1
		s.clampCursor()
	case 'H', 'f': // CUP / HVP
		s.moveCursor(param(1, 1)-1, param(0, 1)-1)
	case 'J': // ED
		switch param(0, 0) {
		case 0:
			s.clearScreen(s.curY, s.curX, s.Rows-1, s.Cols-1)
		case 1:
			s.clearScreen(0, 0, s.curY, s.curX)
		case 2, 3:
			s.clearScreen(0, 0, s.Rows-1, s.Cols-1)
			s.moveCursor(0, 0)
		}
	case 'K': // EL
		switch param(0, 0) {
		case 0:
			s.clearLine(s.curY, s.curX, s.Cols-1)
		case 1:
			s.clearLine(s.curY, 0, s.curX)
		case 2:
			s.clearLine(s.curY, 0, s.Cols-1)
		}
	case 'L': // IL insert lines
		n := param(0, 1)
		for i := 0; i < n; i++ {
			s.cells = append(s.cells[:s.curY],
				append([][]Cell{makeRow(s.Cols)}, s.cells[s.curY:s.Rows-1]...)...)
		}
	case 'M': // DL delete lines
		n := param(0, 1)
		for i := 0; i < n; i++ {
			if s.curY < s.Rows {
				s.cells = append(s.cells[:s.curY],
					append(s.cells[s.curY+1:], makeRow(s.Cols))...)
			}
		}
	case 'P': // DCH
		n := param(0, 1)
		row := s.cells[s.curY]
		if s.curX+n < len(row) {
			copy(row[s.curX:], row[s.curX+n:])
			for i := len(row) - n; i < len(row); i++ {
				row[i] = Cell{Ch: ' ', Style: lipgloss.NewStyle()}
			}
		}
	case 'S': // SU scroll up
		n := param(0, 1)
		for i := 0; i < n; i++ {
			s.scrollUp()
		}
	case 's': // SCP
		s.saveCursor()
	case 'u': // RCP
		s.restoreCursor()
	case 'm': // SGR
		p.applySGR(params)
	case 'h', 'l': // DEC private mode set/reset — mostly ignore
	}
}

func (p *parser) applySGR(params ansi.Params) {
	s := p.screen
	style := s.curStyle

	// Collect all param values into a slice for multi-value parsing (38;5;n, etc.)
	var vals []int
	params.ForEach(0, func(i, param int, hasMore bool) {
		vals = append(vals, param)
	})
	if len(vals) == 0 {
		vals = []int{0}
	}

	i := 0
	for i < len(vals) {
		n := vals[i]
		switch {
		case n == 0:
			style = lipgloss.NewStyle()
		case n == 1:
			style = style.Bold(true)
		case n == 2:
			style = style.Faint(true)
		case n == 3:
			style = style.Italic(true)
		case n == 4:
			style = style.Underline(true)
		case n == 5 || n == 6:
			style = style.Blink(true)
		case n == 7:
			style = style.Reverse(true)
		case n == 9:
			style = style.Strikethrough(true)
		case n == 22:
			style = style.Bold(false).Faint(false)
		case n == 24:
			style = style.Underline(false)
		case n == 27:
			style = style.Reverse(false)
		case n >= 30 && n <= 37:
			style = style.Foreground(ansiColors[n-30])
		case n == 38 && i+2 < len(vals) && vals[i+1] == 5:
			style = style.Foreground(sgrColor(vals[i+2]))
			i += 2
		case n == 38 && i+4 < len(vals) && vals[i+1] == 2:
			style = style.Foreground(sgrTrueColor(vals[i+2], vals[i+3], vals[i+4]))
			i += 4
		case n == 39:
			style = style.UnsetForeground()
		case n >= 40 && n <= 47:
			style = style.Background(ansiColors[n-40])
		case n == 48 && i+2 < len(vals) && vals[i+1] == 5:
			style = style.Background(sgrColor(vals[i+2]))
			i += 2
		case n == 48 && i+4 < len(vals) && vals[i+1] == 2:
			style = style.Background(sgrTrueColor(vals[i+2], vals[i+3], vals[i+4]))
			i += 4
		case n == 49:
			style = style.UnsetBackground()
		case n >= 90 && n <= 97:
			style = style.Foreground(sgrColor(n - 90 + 8))
		case n >= 100 && n <= 107:
			style = style.Background(sgrColor(n - 100 + 8))
		}
		i++
	}
	s.curStyle = style
}
