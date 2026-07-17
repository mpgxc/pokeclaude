package dex

import (
	"bufio"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/mpgxc/pokeclaude/assets"
)

// Frame is a single animation frame: a small block of text lines. Width is the
// display width of the widest line (in terminal columns, ASCII only).
type Frame struct {
	Lines []string
	W     int
	H     int
}

// Species is one Pokémon: a name, a color and its animation frames.
type Species struct {
	Name  string
	Color string // lipgloss/ANSI 256 color code, e.g. "208"
	Idle  Frame
	WalkA Frame
	WalkB Frame
}

// mirrorSwap maps a rune to its horizontal mirror image, used when a sprite
// faces left. Runes not in the table are left unchanged.
var mirrorSwap = map[rune]rune{
	'(': ')', ')': '(',
	'[': ']', ']': '[',
	'{': '}', '}': '{',
	'<': '>', '>': '<',
	'/': '\\', '\\': '/',
	'd': 'b', 'b': 'd',
	'J': 'L', 'L': 'J',
}

// Mirror returns the horizontally flipped copy of a frame.
func (f Frame) Mirror() Frame {
	out := Frame{Lines: make([]string, len(f.Lines)), W: f.W, H: f.H}
	for i, line := range f.Lines {
		runes := []rune(line)
		// Pad to frame width so the flip stays aligned, then reverse.
		for len(runes) < f.W {
			runes = append(runes, ' ')
		}
		flipped := make([]rune, len(runes))
		for j, r := range runes {
			m, ok := mirrorSwap[r]
			if !ok {
				m = r
			}
			flipped[len(runes)-1-j] = m
		}
		out.Lines[i] = string(flipped)
	}
	return out
}

// speciesList is populated at init from the embedded sprite files, sorted by
// name so ordering is deterministic across builds.
var speciesList []Species

func init() {
	entries, err := fs.ReadDir(assets.Sprites, "sprites")
	if err != nil {
		panic(fmt.Sprintf("dex: reading embedded sprites: %v", err))
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		data, err := assets.Sprites.ReadFile("sprites/" + name)
		if err != nil {
			panic(fmt.Sprintf("dex: reading %s: %v", name, err))
		}
		sp, err := parseSpecies(string(data))
		if err != nil {
			panic(fmt.Sprintf("dex: parsing %s: %v", name, err))
		}
		speciesList = append(speciesList, sp)
	}
	if len(speciesList) == 0 {
		panic("dex: no sprites embedded")
	}
}

// parseSpecies parses the sprite file format:
//
//	# name: charmander
//	# color: 208
//	# frames: 3
//	--- idle
//	 (>_<)
//	 ...
//	--- walk-a
//	 ...
func parseSpecies(text string) (Species, error) {
	var sp Species
	frames := map[string]*Frame{}
	var cur *Frame
	var curName string

	flush := func() {
		if cur != nil {
			finalizeFrame(cur)
			frames[curName] = cur
		}
	}

	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "# name:"):
			sp.Name = strings.TrimSpace(strings.TrimPrefix(line, "# name:"))
		case strings.HasPrefix(line, "# color:"):
			sp.Color = strings.TrimSpace(strings.TrimPrefix(line, "# color:"))
		case strings.HasPrefix(line, "# frames:"):
			// informational only
		case strings.HasPrefix(line, "#"):
			// comment
		case strings.HasPrefix(line, "--- "):
			flush()
			curName = strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			cur = &Frame{}
		default:
			if cur != nil {
				cur.Lines = append(cur.Lines, line)
			}
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return sp, err
	}

	if sp.Name == "" {
		return sp, fmt.Errorf("missing name")
	}
	if sp.Color == "" {
		sp.Color = "252"
	}
	idle, ok := frames["idle"]
	if !ok {
		return sp, fmt.Errorf("missing idle frame")
	}
	sp.Idle = *idle
	// walk frames fall back to idle if absent.
	if w, ok := frames["walk-a"]; ok {
		sp.WalkA = *w
	} else {
		sp.WalkA = *idle
	}
	if w, ok := frames["walk-b"]; ok {
		sp.WalkB = *w
	} else {
		sp.WalkB = *idle
	}
	return sp, nil
}

// finalizeFrame trims trailing blank lines and computes width/height.
func finalizeFrame(f *Frame) {
	for len(f.Lines) > 0 && strings.TrimSpace(f.Lines[len(f.Lines)-1]) == "" {
		f.Lines = f.Lines[:len(f.Lines)-1]
	}
	w := 0
	for _, l := range f.Lines {
		if n := len([]rune(l)); n > w {
			w = n
		}
	}
	f.W = w
	f.H = len(f.Lines)
}
