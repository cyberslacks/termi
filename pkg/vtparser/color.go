package vtparser

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// sgrColor maps an xterm-256 color index to a lipgloss.Color.
func sgrColor(index int) lipgloss.Color {
	// 0-15: standard + high-intensity (named by index)
	// 16-231: 6x6x6 color cube
	// 232-255: grayscale
	return lipgloss.Color(fmt.Sprintf("%d", index))
}

// sgrTrueColor builds a lipgloss.Color from RGB values.
func sgrTrueColor(r, g, b int) lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

// ansiColors maps the basic 8 ANSI foreground codes (30-37) to lipgloss colors.
var ansiColors = [8]lipgloss.Color{
	"0",  // black
	"1",  // red
	"2",  // green
	"3",  // yellow
	"4",  // blue
	"5",  // magenta
	"6",  // cyan
	"7",  // white
}
