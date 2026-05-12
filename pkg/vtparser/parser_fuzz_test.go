package vtparser

import "testing"

func FuzzVTParser(f *testing.F) {
	f.Add([]byte("\x1b[32mhello\x1b[0m"))
	f.Add([]byte("\x1b[1;33mworld\r\n"))
	f.Add([]byte("\x1b[2J\x1b[H"))      // clear screen, home
	f.Add([]byte("\x1b[38;5;200mfoo"))  // 256-color fg
	f.Add([]byte("\x1b[48;2;255;0;0m")) // true-color bg
	f.Add([]byte("\x1b[?25l"))          // DEC private mode (ignored)
	f.Add([]byte("\r\n\t\b"))

	f.Fuzz(func(t *testing.T, data []byte) {
		s := NewScreen(80, 24)
		_, _ = s.Write(data) // must not panic regardless of input
	})
}

func TestBasicOutput(t *testing.T) {
	s := NewScreen(80, 24)
	s.Write([]byte("hello")) //nolint:errcheck
	if got := s.CellAt(0, 0).Ch; got != 'h' {
		t.Errorf("cell(0,0) = %q, want 'h'", got)
	}
	if got := s.CellAt(0, 4).Ch; got != 'o' {
		t.Errorf("cell(0,4) = %q, want 'o'", got)
	}
}

func TestClearScreen(t *testing.T) {
	s := NewScreen(80, 24)
	s.Write([]byte("hello\x1b[2J")) //nolint:errcheck
	// After clear, cell 0,0 should be space
	if got := s.CellAt(0, 0).Ch; got != ' ' && got != 0 {
		t.Errorf("after clear, cell(0,0) = %q, want space", got)
	}
}

func TestCursorMovement(t *testing.T) {
	s := NewScreen(80, 24)
	s.Write([]byte("\x1b[5;10H")) //nolint:errcheck // move to row 5, col 10 (1-based)
	col, row := s.Cursor()
	if col != 9 || row != 4 {
		t.Errorf("cursor = (%d,%d), want (9,4)", col, row)
	}
}
