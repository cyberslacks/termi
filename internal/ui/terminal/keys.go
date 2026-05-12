package terminal

import (
	tea "github.com/charmbracelet/bubbletea"
)

// keyToSSHBytes converts a Bubbletea KeyMsg to the raw bytes a terminal sends.
// This is critical so that arrow keys, ctrl sequences, function keys, etc.
// all reach the remote shell correctly rather than as literal strings.
func keyToSSHBytes(msg tea.KeyMsg) []byte {
	if msg.Type == tea.KeyRunes {
		out := make([]byte, 0, 4*len(msg.Runes))
		for _, r := range msg.Runes {
			out = appendRune(out, r)
		}
		if msg.Alt && len(out) > 0 {
			return append([]byte{0x1b}, out...)
		}
		return out
	}

	if b, ok := specialKeys[msg.Type]; ok {
		if msg.Alt {
			return append([]byte{0x1b}, b...)
		}
		// Return a copy so callers can't mutate the table entry
		cp := make([]byte, len(b))
		copy(cp, b)
		return cp
	}
	return nil
}

func appendRune(buf []byte, r rune) []byte {
	if r < 0x80 {
		return append(buf, byte(r))
	}
	var tmp [4]byte
	n := encodeRune(tmp[:], r)
	return append(buf, tmp[:n]...)
}

func encodeRune(p []byte, r rune) int {
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

// specialKeys maps bubbletea key types to the raw bytes a VT terminal sends.
// Note: KeyTab == KeyCtrlI and KeyEnter == KeyCtrlM (same iota value),
// so only one entry per underlying value is kept.
var specialKeys = map[tea.KeyType][]byte{
	tea.KeyEnter:     {0x0d},
	tea.KeyTab:       {0x09},
	tea.KeyBackspace: {0x7f},
	tea.KeyDelete:    {0x1b, '[', '3', '~'},
	tea.KeyEscape:    {0x1b},
	tea.KeySpace:     {0x20},

	tea.KeyUp:    {0x1b, '[', 'A'},
	tea.KeyDown:  {0x1b, '[', 'B'},
	tea.KeyRight: {0x1b, '[', 'C'},
	tea.KeyLeft:  {0x1b, '[', 'D'},

	tea.KeyShiftUp:   {0x1b, '[', '1', ';', '2', 'A'},
	tea.KeyShiftDown: {0x1b, '[', '1', ';', '2', 'B'},

	tea.KeyHome:  {0x1b, '[', 'H'},
	tea.KeyEnd:   {0x1b, '[', 'F'},
	tea.KeyPgUp:  {0x1b, '[', '5', '~'},
	tea.KeyPgDown: {0x1b, '[', '6', '~'},

	tea.KeyF1:  {0x1b, 'O', 'P'},
	tea.KeyF2:  {0x1b, 'O', 'Q'},
	tea.KeyF3:  {0x1b, 'O', 'R'},
	tea.KeyF4:  {0x1b, 'O', 'S'},
	tea.KeyF5:  {0x1b, '[', '1', '5', '~'},
	tea.KeyF6:  {0x1b, '[', '1', '7', '~'},
	tea.KeyF7:  {0x1b, '[', '1', '8', '~'},
	tea.KeyF8:  {0x1b, '[', '1', '9', '~'},
	tea.KeyF9:  {0x1b, '[', '2', '0', '~'},
	tea.KeyF10: {0x1b, '[', '2', '1', '~'},
	tea.KeyF11: {0x1b, '[', '2', '3', '~'},
	tea.KeyF12: {0x1b, '[', '2', '4', '~'},

	// Ctrl+letter keys (0x01–0x1A).
	// KeyCtrlI == KeyTab and KeyCtrlM == KeyEnter (same iota), omitted here.
	tea.KeyCtrlA: {0x01},
	tea.KeyCtrlB: {0x02},
	tea.KeyCtrlC: {0x03},
	tea.KeyCtrlD: {0x04},
	tea.KeyCtrlE: {0x05},
	tea.KeyCtrlF: {0x06},
	tea.KeyCtrlG: {0x07},
	tea.KeyCtrlH: {0x08},
	tea.KeyCtrlJ: {0x0a},
	tea.KeyCtrlK: {0x0b},
	tea.KeyCtrlL: {0x0c},
	tea.KeyCtrlN: {0x0e},
	tea.KeyCtrlO: {0x0f},
	tea.KeyCtrlP: {0x10},
	tea.KeyCtrlQ: {0x11},
	tea.KeyCtrlR: {0x12},
	tea.KeyCtrlS: {0x13},
	tea.KeyCtrlT: {0x14},
	tea.KeyCtrlU: {0x15},
	tea.KeyCtrlV: {0x16},
	tea.KeyCtrlW: {0x17},
	tea.KeyCtrlX: {0x18},
	tea.KeyCtrlY: {0x19},
	tea.KeyCtrlZ: {0x1a},

	tea.KeyCtrlBackslash:  {0x1c},
	tea.KeyCtrlUnderscore: {0x1f},
}
