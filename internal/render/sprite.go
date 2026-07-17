package render

import (
	"github.com/mpgxc/pokeclaude/internal/dex"
	"github.com/mpgxc/pokeclaude/internal/world"
)

// frameFor picks the animation frame for an agent, applying horizontal mirror
// when it faces left.
func frameFor(av world.AgentView) dex.Frame {
	var fr dex.Frame
	switch av.WalkFrame {
	case 1:
		fr = av.Species.WalkA
	case 2:
		fr = av.Species.WalkB
	default:
		fr = av.Species.Idle
	}
	if av.Facing < 0 {
		fr = fr.Mirror()
	}
	return fr
}

// blitSprite draws an agent's sprite into the frame at its position.
func blitSprite(f *Frame, av world.AgentView, clip world.Rect) {
	x0 := int(av.Pos.X + 0.5)
	y0 := int(av.Pos.Y + 0.5)
	st := Style{Fg: av.Species.Color, Bold: true}

	if av.IsSub {
		blitSub(f, av, x0, y0, st, clip)
		return
	}
	fr := frameFor(av)
	for dy, line := range fr.Lines {
		x := x0
		for _, r := range line {
			if r != ' ' && inClip(x, y0+dy, clip) {
				f.Set(x, y0+dy, r, st)
			}
			x++
		}
	}
}

// blitSub draws the reduced subagent sprite: a "°" antenna over the middle
// body line of the species' idle frame.
func blitSub(f *Frame, av world.AgentView, x0, y0 int, st Style, clip world.Rect) {
	idle := av.Species.Idle
	mid := ""
	if len(idle.Lines) > 0 {
		mid = idle.Lines[len(idle.Lines)/2]
	}
	if av.Facing < 0 {
		m := dex.Frame{Lines: []string{mid}, W: idle.W, H: 1}.Mirror()
		mid = m.Lines[0]
	}
	if inClip(x0+1, y0, clip) {
		f.Set(x0+1, y0, '°', st)
	}
	x := x0
	for _, r := range mid {
		if r != ' ' && inClip(x, y0+1, clip) {
			f.Set(x, y0+1, r, st)
		}
		x++
	}
}

// inClip reports whether (x,y) lies inside the clip rect (a zero rect = no clip).
func inClip(x, y int, clip world.Rect) bool {
	if clip.W == 0 && clip.H == 0 {
		return true
	}
	return clip.Contains(x, y)
}
