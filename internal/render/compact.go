package render

import (
	"fmt"
	"sort"

	"github.com/mpgxc/pokeclaude/internal/world"
)

// renderCompact is the fallback for terminals below 80×24: no map, just a
// header and a one-line-per-agent list.
func renderCompact(v world.View, w, h int) string {
	f := NewFrame(w, h)

	title := fmt.Sprintf("PokéClaude · %d agentes · %d zonas", len(v.Agents), v.ZoneTotal)
	f.SetString(0, 0, truncWidth(title, w), titleStyle)
	clock := v.Now.Format("15:04:05")
	if w > len(clock)+2 {
		f.SetString(w-len(clock), 0, clock, dimStyle)
	}

	zoneLabel := map[string]string{}
	for _, z := range v.Zones {
		zoneLabel[z.ID] = z.Label
	}

	agents := make([]world.AgentView, len(v.Agents))
	copy(agents, v.Agents)
	sort.SliceStable(agents, func(i, j int) bool { return agents[i].LastSeen.After(agents[j].LastSeen) })

	row := 2
	for _, a := range agents {
		if row >= h {
			break
		}
		dot := Style{Fg: stateColors[a.State]}
		f.Set(0, row, '●', dot)
		x := 2
		x = f.SetString(x, row, padRightWidth(a.Nick, 12), Style{Fg: a.Species.Color})
		x++
		x = f.SetString(x, row, padRightWidth(shorten(zoneLabel[a.ZoneID], 16), 16), dimStyle)
		x++
		x = f.SetString(x, row, padRightWidth(a.State.String(), 9), dot)
		x++
		detail := footerDetail(a)
		f.SetString(x, row, truncWidth(detail, maxInt(1, w-x)), dimStyle)
		row++
	}
	if len(agents) == 0 {
		f.SetString(0, 2, "aguardando sessões do Claude Code…", dimStyle)
	}
	return f.Render()
}

func shorten(s string, w int) string {
	return truncWidth(s, w)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
