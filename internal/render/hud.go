package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/mpgxc/pokeclaude/internal/world"
)

// HUD chrome sizing. The footer always reserves footerLines rows so the map
// area stays a fixed size regardless of how many agents are listed.
const (
	footerLines = 5
	chromeRows  = 3 + 1 + footerLines + 1 // top+header+sep + sep + footer + bottom
	sideBorders = 2
)

var (
	frameStyle  = Style{Fg: "63"}
	titleStyle  = Style{Fg: "213", Bold: true}
	dimStyle    = Style{Fg: "245"}
	stateColors = map[world.State]string{
		world.StateIdle:     "245",
		world.StateThinking: "111",
		world.StateWorking:  "154",
		world.StateError:    "203",
		world.StateDone:     "84",
	}
)

// Layout describes where the HUD chrome and the map area sit for a terminal of
// the given size.
type Layout struct {
	Map                     world.Rect
	TopY, HeaderY, Sep1Y    int
	Sep2Y, FooterY, BottomY int
	W                       int
}

// ComputeLayout returns the HUD layout for a w×h terminal. The map area is the
// interior; everything else is chrome.
func ComputeLayout(w, h int) Layout {
	mapH := h - chromeRows
	if mapH < 1 {
		mapH = 1
	}
	mapW := w - sideBorders
	if mapW < 1 {
		mapW = 1
	}
	return Layout{
		Map:     world.Rect{X: 1, Y: 3, W: mapW, H: mapH},
		TopY:    0,
		HeaderY: 1,
		Sep1Y:   2,
		Sep2Y:   3 + mapH,
		FooterY: 3 + mapH + 1,
		BottomY: 3 + mapH + 1 + footerLines,
		W:       w,
	}
}

// RenderFrame renders the whole screen (chrome + map, or the compact fallback)
// for a terminal of size w×h.
func RenderFrame(v world.View, w, h int) string {
	if w < 80 || h < 24 {
		return renderCompact(v, w, h)
	}
	lay := ComputeLayout(w, h)
	f := NewFrame(w, h)
	drawChrome(f, v, lay)
	drawMap(f, v)
	return f.Render()
}

// drawChrome draws the outer box, header line and footer agent list.
func drawChrome(f *Frame, v world.View, lay Layout) {
	w := lay.W
	// horizontal borders
	f.SetString(0, lay.TopY, "╔"+strings.Repeat("═", w-2)+"╗", frameStyle)
	f.SetString(0, lay.Sep1Y, "╠"+strings.Repeat("═", w-2)+"╣", frameStyle)
	f.SetString(0, lay.Sep2Y, "╠"+strings.Repeat("═", w-2)+"╣", frameStyle)
	f.SetString(0, lay.BottomY, "╚"+strings.Repeat("═", w-2)+"╝", frameStyle)
	// vertical borders for header + footer rows
	for _, y := range []int{lay.HeaderY} {
		f.Set(0, y, '║', frameStyle)
		f.Set(w-1, y, '║', frameStyle)
	}
	for y := lay.FooterY; y < lay.BottomY; y++ {
		f.Set(0, y, '║', frameStyle)
		f.Set(w-1, y, '║', frameStyle)
	}
	// map side borders
	for y := lay.Map.Y; y < lay.Map.Bottom(); y++ {
		f.Set(0, y, '║', frameStyle)
		f.Set(w-1, y, '║', frameStyle)
	}

	drawHeader(f, v, lay)
	drawFooter(f, v, lay)
}

func drawHeader(f *Frame, v world.View, lay Layout) {
	f.SetString(2, lay.HeaderY, "PokéClaude", titleStyle)
	counts := fmt.Sprintf("%d agentes · %d zonas", len(v.Agents), v.ZoneTotal)
	f.SetString(14, lay.HeaderY, counts, dimStyle)
	if v.PageCount > 1 {
		pg := fmt.Sprintf("[pág %d/%d]", v.Page+1, v.PageCount)
		f.SetString(14+runewidth.StringWidth(counts)+2, lay.HeaderY, pg, dimStyle)
	}
	clock := v.Now.Format("15:04:05")
	f.SetString(lay.W-2-runewidth.StringWidth(clock), lay.HeaderY, clock, dimStyle)
}

func drawFooter(f *Frame, v world.View, lay Layout) {
	agents := make([]world.AgentView, len(v.Agents))
	copy(agents, v.Agents)
	sort.SliceStable(agents, func(i, j int) bool { return agents[i].LastSeen.After(agents[j].LastSeen) })
	if len(agents) > footerLines {
		agents = agents[:footerLines]
	}
	for i, a := range agents {
		y := lay.FooterY + i
		nick := a.Nick
		if a.IsSub {
			nick = "› " + nick
		}
		f.SetString(2, y, padRightWidth(nick, 12), Style{Fg: a.Species.Color})
		f.SetString(15, y, padRightWidth(a.State.String(), 9), Style{Fg: stateColors[a.State]})
		detail := footerDetail(a)
		maxDetail := lay.W - 2 - 25
		if maxDetail < 1 {
			maxDetail = 1
		}
		f.SetString(25, y, truncWidth(detail, maxDetail), dimStyle)
	}
}

// footerDetail returns the human-readable activity string for an agent.
func footerDetail(a world.AgentView) string {
	switch a.State {
	case world.StateWorking:
		if a.LastTool != "" {
			return a.LastTool + ": " + a.Thought.Text
		}
		return a.Thought.Text
	case world.StateThinking:
		if a.Thought.Text != "" {
			return "\"" + a.Thought.Text + "\""
		}
		return "pensando…"
	default:
		return a.Thought.Text
	}
}
