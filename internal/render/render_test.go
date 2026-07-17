package render

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// TestMain forces a plain color profile so Render output is escape-free and
// golden comparisons are stable.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func TestFrameWideRuneAlignment(t *testing.T) {
	f := NewFrame(5, 1)
	f.Set(0, 0, 'A', Style{})
	f.Set(1, 0, '世', Style{}) // width 2
	f.Set(3, 0, 'B', Style{})
	got := f.Render()
	const want = "A世B "
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
	if runewidth.StringWidth(got) != 5 {
		t.Errorf("display width = %d, want 5", runewidth.StringWidth(got))
	}
}

func TestFrameWideRuneOverwriteRepairs(t *testing.T) {
	f := NewFrame(4, 1)
	f.Set(0, 0, '世', Style{}) // occupies 0,1 (ghost at 1)
	f.Set(1, 0, 'X', Style{}) // overwrites the ghost; head at 0 must be blanked
	got := f.Render()
	const want = " X  "
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestSetStringAdvancesByWidth(t *testing.T) {
	f := NewFrame(10, 1)
	end := f.SetString(0, 0, "a世b", Style{})
	if end != 4 { // 1 + 2 + 1
		t.Errorf("end col = %d, want 4", end)
	}
}

func TestBubbleLinesAlign(t *testing.T) {
	art := BubbleArt('⚡', "npm test", 6, false)
	// borders and content lines (all but the tail line) must share one width
	w := runewidth.StringWidth(art[0])
	for i := 0; i < len(art)-1; i++ {
		if got := runewidth.StringWidth(art[i]); got != w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, w, art[i])
		}
	}
}

func TestBubbleWrapsToTwoLines(t *testing.T) {
	art := BubbleArt(0, "adicionar retry no consumer de boletos agora", 4, false)
	// top + 2 content + bottom + tail = 5 lines
	if len(art) != 5 {
		t.Errorf("lines = %d, want 5 (two-line wrap): %v", len(art), art)
	}
}

func TestBubbleBelowInvertsTail(t *testing.T) {
	below := BubbleArt('💭', "oi", 3, true)
	// in below mode the tail line comes first
	if runewidth.StringWidth(below[0]) >= runewidth.StringWidth(below[1]) {
		t.Errorf("expected short tail line first, got %q then %q", below[0], below[1])
	}
}
