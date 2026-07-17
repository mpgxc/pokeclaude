// Package render turns a world snapshot into a single string frame for the TUI.
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Style is a minimal cell style: a foreground color (ANSI-256 code as a string,
// "" = default) and a bold flag. Keeping it comparable lets the renderer group
// runs of identically-styled cells into a single escape sequence.
type Style struct {
	Fg   string
	Bold bool
}

// Cell is one terminal cell in the framebuffer.
type Cell struct {
	R     rune
	St    Style
	ghost bool // right half of a wide (2-column) rune; emitted as nothing
}

// Frame is a fixed-size framebuffer.
type Frame struct {
	W, H  int
	cells []Cell
}

// NewFrame allocates a blank frame filled with spaces.
func NewFrame(w, h int) *Frame {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	cells := make([]Cell, w*h)
	for i := range cells {
		cells[i].R = ' '
	}
	return &Frame{W: w, H: h, cells: cells}
}

// Set writes a single rune at (x,y), handling wide runes by marking the
// following cell as a ghost and repairing any wide rune it splits.
func (f *Frame) Set(x, y int, r rune, st Style) {
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return
	}
	i := y*f.W + x

	// If we overwrite the tail (ghost) of a wide rune to our left, blank its head.
	if f.cells[i].ghost && x > 0 {
		f.cells[i-1] = Cell{R: ' '}
	}
	// If we overwrite the head of a wide rune, blank its trailing ghost.
	if runewidth.RuneWidth(f.cells[i].R) == 2 && !f.cells[i].ghost && x+1 < f.W {
		f.cells[i+1] = Cell{R: ' '}
	}

	w := runewidth.RuneWidth(r)
	if w <= 0 {
		w = 1
	}
	f.cells[i] = Cell{R: r, St: st}
	if w == 2 && x+1 < f.W {
		j := i + 1
		// blank a wide head that our ghost would split
		if runewidth.RuneWidth(f.cells[j].R) == 2 && !f.cells[j].ghost && x+2 < f.W {
			f.cells[j+1] = Cell{R: ' '}
		}
		f.cells[j] = Cell{R: ' ', St: st, ghost: true}
	}
}

// SetString writes s starting at (x,y), advancing by each rune's display width.
// It returns the x column just past the written text.
func (f *Frame) SetString(x, y int, s string, st Style) int {
	for _, r := range s {
		f.Set(x, y, r, st)
		x += runewidth.RuneWidth(r)
	}
	return x
}

// Render produces the final multi-line string with ANSI styling, grouping
// consecutive cells that share a style.
func (f *Frame) Render() string {
	var sb strings.Builder
	for y := 0; y < f.H; y++ {
		var run strings.Builder
		var cur Style
		flush := func() {
			if run.Len() == 0 {
				return
			}
			sb.WriteString(styleFor(cur).Render(run.String()))
			run.Reset()
		}
		for x := 0; x < f.W; x++ {
			c := f.cells[y*f.W+x]
			if c.ghost {
				continue
			}
			if c.St != cur {
				flush()
				cur = c.St
			}
			run.WriteRune(c.R)
		}
		flush()
		if y < f.H-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// styleFor builds a lipgloss style from a Style value.
func styleFor(s Style) lipgloss.Style {
	st := lipgloss.NewStyle()
	if s.Fg != "" {
		st = st.Foreground(lipgloss.Color(s.Fg))
	}
	if s.Bold {
		st = st.Bold(true)
	}
	return st
}
