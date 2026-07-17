package render

import (
	"math"

	"github.com/mpgxc/pokeclaude/internal/world"
)

// Space Drift palette.
var (
	starDim   = Style{Fg: "238"}
	starMid   = Style{Fg: "244"}
	starFast  = Style{Fg: "250"}
	warpTrail = Style{Fg: "45"}
	warpHot   = Style{Fg: "51", Bold: true}
	laserSt   = Style{Fg: "227", Bold: true}
	cometHead = Style{Fg: "231", Bold: true}
	cometTail = Style{Fg: "44"}
	astSmall  = Style{Fg: "137"}
	astBig    = Style{Fg: "179"}
	flameSt   = Style{Fg: "208", Bold: true}
	hitSt     = Style{Fg: "196", Bold: true}
	shipWarp  = Style{Fg: "51", Bold: true}
)

// drawSpace renders the Space Drift scene into the map area of the frame.
func drawSpace(f *Frame, v world.View) {
	sp := v.Space
	b := sp.Bounds
	put := func(x, y int, r rune, st Style) {
		if b.Contains(x, y) {
			f.Set(x, y, r, st)
		}
	}

	drawStarfield(put, sp)
	drawObstacles(put, sp)
	drawProjectiles(put, sp)
	drawExplosions(put, sp)
	drawShips(put, sp)
}

// drawStarfield paints the scrolling background and the warp speed-lines.
func drawStarfield(put func(int, int, rune, Style), sp world.SpaceView) {
	warp := sp.Warp
	for _, s := range sp.Stars {
		x, y := int(s.Pos.X), int(s.Pos.Y)

		trail := 0
		tcol := starDim
		switch {
		case warp > 0:
			trail = int(2 + warp*6*s.Speed)
			tcol = warpTrail
		case s.Speed > 1.0:
			trail = 1
		}
		for t := 1; t <= trail; t++ {
			put(x+t, y, '─', tcol)
		}

		g, st := starGlyph(s.Speed, warp)
		put(x, y, g, st)
	}
}

func starGlyph(speed, warp float32) (rune, Style) {
	if warp > 0 {
		return '*', warpHot
	}
	switch {
	case speed > 1.1:
		return '*', starFast
	case speed > 0.7:
		return '·', starMid
	default:
		return '.', starDim
	}
}

func drawObstacles(put func(int, int, rune, Style), sp world.SpaceView) {
	for _, o := range sp.Obstacles {
		x, y := round(o.Pos.X), round(o.Pos.Y)
		switch o.Kind {
		case world.Asteroid:
			if o.Size >= 2 {
				put(x-1, y, '(', astBig)
				put(x, y, '@', astBig)
				put(x+1, y, ')', astBig)
			} else {
				put(x, y, 'o', astSmall)
			}
		case world.Comet:
			put(x, y, '*', cometHead)
			// tail trails opposite the velocity
			dx, dy := unit(-o.Vel.X, -o.Vel.Y)
			for t := 1; t <= 3; t++ {
				tx := x + round(dx*float32(t))
				ty := y + round(dy*float32(t))
				ch := ':'
				if t == 3 {
					ch = '.'
				}
				put(tx, ty, ch, cometTail)
			}
		}
	}
}

func drawProjectiles(put func(int, int, rune, Style), sp world.SpaceView) {
	for _, p := range sp.Projectiles {
		put(round(p.Pos.X), round(p.Pos.Y), laserGlyph(p.Vel), laserSt)
	}
}

func laserGlyph(vel world.Vec2) rune {
	ax, ay := absf32(vel.X), absf32(vel.Y)
	if ax >= ay*2 {
		return '-'
	}
	if ay >= ax*2 {
		return '|'
	}
	if (vel.X > 0) == (vel.Y < 0) {
		return '/'
	}
	return '\\'
}

func drawExplosions(put func(int, int, rune, Style), sp world.SpaceView) {
	for _, e := range sp.Explosions {
		x, y := round(e.Pos.X), round(e.Pos.Y)
		g, st := explosionGlyph(e.Frame)
		put(x, y, g, st)
		if e.Big && e.Frame < 2 {
			put(x-1, y, '-', st)
			put(x+1, y, '-', st)
			put(x, y-1, '|', st)
			put(x, y+1, '|', st)
		}
	}
}

func explosionGlyph(frame int) (rune, Style) {
	switch frame {
	case 0:
		return '*', Style{Fg: "231", Bold: true}
	case 1:
		return '+', Style{Fg: "214", Bold: true}
	default:
		return '·', Style{Fg: "240"}
	}
}

func drawShips(put func(int, int, rune, Style), sp world.SpaceView) {
	for _, s := range sp.Ships {
		x, y := round(s.Pos.X), round(s.Pos.Y)

		// warp jump: draw a stretch streak; hide the hull mid-tunnel
		if s.Warp > 0.15 {
			dx, dy := unit(-s.Vel.X, -s.Vel.Y)
			n := int(3 + s.Warp*6)
			for t := 0; t <= n; t++ {
				put(x+round(dx*float32(t)), y+round(dy*float32(t)), '=', shipWarp)
			}
			if s.Warp > 0.5 {
				continue
			}
		}

		col := s.Species.Color
		st := Style{Fg: col, Bold: true}
		if s.Hit {
			st = hitSt
		}

		if s.IsSub {
			put(x, y, subGlyph(s.Vel), st)
		} else {
			drawShipHull(put, x, y, s.Vel, st, s.Boosting)
		}

		// small dim callsign to the right
		label := s.Nick
		if len(label) > 8 {
			label = label[:8]
		}
		lx := x + 2
		for _, r := range label {
			put(lx, y, r, Style{Fg: "240"})
			lx++
		}
	}
}

// drawShipHull draws the 3-cell craft oriented by velocity, plus a thruster
// flame behind it when boosting.
func drawShipHull(put func(int, int, rune, Style), x, y int, vel world.Vec2, st Style, boost bool) {
	l, m, r, dir := hullCells(vel)
	put(x-1, y, l, st)
	put(x, y, m, st)
	put(x+1, y, r, st)
	if boost {
		switch dir {
		case dirRight:
			put(x-2, y, '≺', flameSt)
		case dirLeft:
			put(x+2, y, '≻', flameSt)
		case dirUp:
			put(x, y+2, 'v', flameSt)
		case dirDown:
			put(x, y-2, '^', flameSt)
		}
	}
}

type shipDir int

const (
	dirRight shipDir = iota
	dirLeft
	dirUp
	dirDown
)

// hullCells returns the three hull runes and the facing direction for a velocity.
func hullCells(vel world.Vec2) (rune, rune, rune, shipDir) {
	ax, ay := absf32(vel.X), absf32(vel.Y)
	if ax >= ay {
		if vel.X >= 0 {
			return '[', '=', '>', dirRight
		}
		return '<', '=', ']', dirLeft
	}
	if vel.Y < 0 {
		return '/', '^', '\\', dirUp
	}
	return '\\', 'v', '/', dirDown
}

func subGlyph(vel world.Vec2) rune {
	ax, ay := absf32(vel.X), absf32(vel.Y)
	if ax >= ay {
		if vel.X >= 0 {
			return '›'
		}
		return '‹'
	}
	if vel.Y < 0 {
		return '^'
	}
	return 'v'
}

// --- small numeric helpers ---

func round(f float32) int { return int(math.Floor(float64(f) + 0.5)) }

func absf32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// unit returns the normalized components of (x,y), or (0,0) if degenerate.
func unit(x, y float32) (float32, float32) {
	l := float32(math.Hypot(float64(x), float64(y)))
	if l < 0.001 {
		return 0, 0
	}
	return x / l, y / l
}
