package render

import (
	"sort"

	"github.com/mpgxc/pokeclaude/internal/world"
)

// zone-kind border colors
var zoneKindColor = map[world.ZoneKind]string{
	world.ZoneGrass: "34",
	world.ZoneLake:  "39",
	world.ZoneCave:  "244",
}

// drawMap renders every zone, its decoration, agents (z-ordered) and bubbles
// into the frame.
func drawMap(f *Frame, v world.View) {
	ms := v.Now.UnixMilli()
	zoneByID := make(map[string]world.ZoneView, len(v.Zones))
	for _, z := range v.Zones {
		zoneByID[z.ID] = z
		drawZone(f, z, ms)
	}

	// z-order: agents lower on the screen draw last (on top)
	agents := make([]world.AgentView, len(v.Agents))
	copy(agents, v.Agents)
	sort.SliceStable(agents, func(i, j int) bool { return agents[i].Pos.Y < agents[j].Pos.Y })

	for _, a := range agents {
		z, ok := zoneByID[a.ZoneID]
		if !ok {
			continue // agent's zone is on another page
		}
		blitSprite(f, a, z.Rect)
	}
	for _, a := range agents {
		z, ok := zoneByID[a.ZoneID]
		if !ok {
			continue
		}
		blitBubble(f, a, z)
	}
}

// drawZone draws a zone's border, label and decoration.
func drawZone(f *Frame, z world.ZoneView, ms int64) {
	col := zoneKindColor[z.Kind]
	st := Style{Fg: col}
	r := z.Rect
	if r.W < 2 || r.H < 2 {
		return
	}
	// corners
	f.Set(r.X, r.Y, '┌', st)
	f.Set(r.Right()-1, r.Y, '┐', st)
	f.Set(r.X, r.Bottom()-1, '└', st)
	f.Set(r.Right()-1, r.Bottom()-1, '┘', st)
	// edges
	for x := r.X + 1; x < r.Right()-1; x++ {
		f.Set(x, r.Y, '─', st)
		f.Set(x, r.Bottom()-1, '─', st)
	}
	for y := r.Y + 1; y < r.Bottom()-1; y++ {
		f.Set(r.X, y, '│', st)
		f.Set(r.Right()-1, y, '│', st)
	}
	// label
	label := " " + z.Label + " "
	f.SetString(r.X+2, r.Y, label, Style{Fg: col, Bold: true})

	drawDeco(f, z, ms)
}

// drawDeco paints a zone's decorative glyphs, animating lake water.
func drawDeco(f *Frame, z world.ZoneView, ms int64) {
	phase := int(ms / 500)
	for _, d := range z.Deco {
		st := Style{Fg: d.Color}
		r := d.Rune
		if z.Kind == world.ZoneLake {
			// shimmer: brighten a moving column of water
			if (d.X+phase)%4 == 0 {
				st.Fg = "51"
			}
		}
		f.Set(d.X, d.Y, r, st)
	}
}
