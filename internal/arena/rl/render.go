//go:build raylib

// Package rl is the raylib renderer for the Arena game mode. It is compiled only
// with the `raylib` build tag so the rest of the project (and its tests) build
// without a graphics stack. All art is procedural and original — no copyrighted
// assets — while aiming for a Mortal-Kombat-style presentation.
package rl

import (
	"fmt"
	"hash/fnv"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/mpgxc/pokeclaude/internal/arena"
)

const (
	screenW = 1280
	screenH = 720
	margin  = 120.0
	groundY = 560.0
	figH    = 150.0
	figW    = 70.0
)

// Run opens the window and drives the match until closed.
func Run(m *arena.Arena) error {
	rl.InitWindow(screenW, screenH, "PokéClaude — Arena")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	for !rl.WindowShouldClose() {
		m.Advance(float64(rl.GetFrameTime()))
		v := m.View()
		draw(v)
	}
	return nil
}

func draw(v arena.MatchView) {
	rl.BeginDrawing()
	defer rl.EndDrawing()

	// screen shake offset
	t := rl.GetTime()
	ox := float32(math.Sin(t*57.0) * v.Shake * 2.2)
	oy := float32(math.Cos(t*61.0) * v.Shake * 1.6)

	drawBackground(ox, oy)
	drawFighter(v.A, v.Width, ox, oy)
	drawFighter(v.B, v.Width, ox, oy)
	drawSparks(v, ox, oy)
	drawHUD(v)
	drawAnnounce(v)
}

func drawBackground(ox, oy float32) {
	rl.ClearBackground(rl.NewColor(12, 10, 18, 255))
	// distant gradient
	for i := int32(0); i < 8; i++ {
		c := rl.NewColor(uint8(20+i*4), uint8(12+i*2), uint8(30+i*5), 255)
		rl.DrawRectangle(0, i*40, screenW, 40, c)
	}
	// cage bars
	for x := int32(60); x < screenW; x += 90 {
		rl.DrawRectangle(x+int32(ox), 60+int32(oy), 6, groundY-60, rl.NewColor(40, 40, 55, 120))
	}
	// ground
	rl.DrawRectangle(0, int32(groundY)+int32(oy), screenW, screenH-int32(groundY), rl.NewColor(26, 20, 24, 255))
	rl.DrawLineEx(
		rl.NewVector2(0, groundY+oy), rl.NewVector2(screenW, groundY+oy), 3,
		rl.NewColor(90, 60, 40, 255))
}

func mapX(x, width float64) float32 {
	return float32(margin + x/width*(screenW-2*margin))
}

func drawFighter(f arena.ViewFighter, width float64, ox, oy float32) {
	x := mapX(f.X, width) + ox
	baseY := float32(groundY) + oy
	col := speciesColor(f.Species.Name)
	dark := shade(col, 0.6)
	face := float32(f.Facing)

	// dodge afterimages
	if f.Dodging {
		for i := 1; i <= 3; i++ {
			gx := x - face*float32(i)*16
			drawBody(gx, baseY, col, dark, face, f, rl.Fade(col, 0.18))
		}
	}

	tint := col
	switch {
	case f.Stunned:
		tint = rl.NewColor(220, 80, 80, 255)
	case f.KO:
		tint = shade(col, 0.4)
	}
	drawBody(x, baseY, tint, dark, face, f, rl.Color{})
}

// drawBody draws one fighter body. If ghost.A != 0 it draws a translucent
// after-image instead of the solid body.
func drawBody(x, baseY float32, col, dark rl.Color, face float32, f arena.ViewFighter, ghost rl.Color) {
	body := col
	if ghost.A != 0 {
		body = ghost
	}

	// shadow
	rl.DrawEllipse(int32(x), int32(baseY)+4, figW*0.55, 10, rl.NewColor(0, 0, 0, 120))

	if f.KO {
		// fallen: horizontal body on the ground
		rl.DrawRectangleRounded(rl.NewRectangle(x-figH*0.5, baseY-figW*0.4, figH, figW*0.5), 0.5, 6, body)
		return
	}

	lean := float32(0)
	if f.Attacking {
		lean = face * 14
	} else if f.Stunned {
		lean = -face * 16
	}

	top := baseY - figH
	// legs
	rl.DrawRectangleRounded(rl.NewRectangle(x-figW*0.32, baseY-figH*0.42, figW*0.24, figH*0.42), 0.4, 4, dark)
	rl.DrawRectangleRounded(rl.NewRectangle(x+figW*0.08, baseY-figH*0.42, figW*0.24, figH*0.42), 0.4, 4, dark)
	// torso
	rl.DrawRectangleRounded(rl.NewRectangle(x-figW*0.3+lean*0.3, top+figH*0.3, figW*0.6, figH*0.42), 0.35, 6, body)
	// head
	rl.DrawCircle(int32(x+lean*0.5), int32(top+figH*0.2), figW*0.28, body)
	// eyes (facing)
	ex := x + lean*0.5 + face*figW*0.12
	rl.DrawCircle(int32(ex), int32(top+figH*0.18), 3.5, rl.NewColor(240, 240, 255, 255))

	drawArms(x, top, baseY, face, dark, f)

	// combo popup
	if f.ComboCount > 1 && f.Attacking {
		txt := fmt.Sprintf("COMBO x%d", f.ComboCount)
		rl.DrawText(txt, int32(x)-30, int32(top)-26, 20, rl.NewColor(255, 210, 60, 255))
	}
}

func drawArms(x, top, baseY, face float32, dark rl.Color, f arena.ViewFighter) {
	shoulder := rl.NewVector2(x, top+figH*0.36)
	switch {
	case f.Blocking:
		// guard: forearm across the front
		gx := x + face*figW*0.34
		rl.DrawRectangleRounded(rl.NewRectangle(gx-8, top+figH*0.3, 16, figH*0.3), 0.5, 4, rl.NewColor(200, 200, 210, 255))
	case f.Attacking:
		reach := float32(figW * 0.9)
		strike := rl.NewColor(255, 90, 60, 255)
		if f.Move == arena.ActSpecial || f.Move == arena.ActSuper {
			strike = rl.NewColor(120, 200, 255, 255)
			reach = float32(figW * 1.2)
		}
		end := rl.NewVector2(x+face*reach, top+figH*0.4)
		rl.DrawLineEx(shoulder, end, 10, dark)
		rl.DrawCircleV(end, 9, strike)
	default:
		end := rl.NewVector2(x+face*figW*0.28, top+figH*0.55)
		rl.DrawLineEx(shoulder, end, 8, dark)
	}
}

func drawSparks(v arena.MatchView, ox, oy float32) {
	for _, s := range v.Sparks {
		x := mapX(s.X, v.Width) + ox
		y := float32(groundY) - float32(s.Y)*8 + oy
		p := 1 - float32(s.Age/s.TTL)
		r := float32(6) + p*14
		col := rl.NewColor(255, 220, 80, uint8(220*p))
		if s.Big {
			col = rl.NewColor(255, 90, 60, uint8(230*p))
		}
		rl.DrawCircle(int32(x), int32(y), r, col)
		// spikes
		for a := 0.0; a < math.Pi*2; a += math.Pi / 4 {
			ex := x + float32(math.Cos(a))*(r+6)
			ey := y + float32(math.Sin(a))*(r+6)
			rl.DrawLineEx(rl.NewVector2(x, y), rl.NewVector2(ex, ey), 2, rl.Fade(col, p))
		}
	}
}

func drawHUD(v arena.MatchView) {
	drawBar(30, 30, 520, true, v.A)
	drawBar(screenW-30-520, 30, 520, false, v.B)

	// timer
	tm := fmt.Sprintf("%02d", int(v.TimeLeft))
	tw := rl.MeasureText(tm, 44)
	rl.DrawRectangle(screenW/2-40, 20, 80, 56, rl.NewColor(0, 0, 0, 160))
	rl.DrawText(tm, screenW/2-tw/2, 26, 44, rl.NewColor(255, 230, 120, 255))

	// round pips
	drawPips(screenW/2-90, 84, v.ScoreA, v.RoundsToWin, true)
	drawPips(screenW/2+90, 84, v.ScoreB, v.RoundsToWin, false)
}

func drawBar(x, y, w int32, leftAligned bool, f arena.ViewFighter) {
	// name
	name := f.Nick
	if leftAligned {
		rl.DrawText(name, x, y-2, 22, rl.White)
	} else {
		tw := rl.MeasureText(name, 22)
		rl.DrawText(name, x+w-tw, y-2, 22, rl.White)
	}
	by := y + 24
	// HP background
	rl.DrawRectangle(x, by, w, 22, rl.NewColor(40, 10, 10, 255))
	hp := float32(f.HP / f.MaxHP)
	if hp < 0 {
		hp = 0
	}
	fillW := int32(float32(w) * hp)
	col := rl.NewColor(230, 200, 40, 255)
	if hp < 0.3 {
		col = rl.NewColor(220, 50, 40, 255)
	}
	if leftAligned {
		rl.DrawRectangle(x, by, fillW, 22, col)
	} else {
		rl.DrawRectangle(x+w-fillW, by, fillW, 22, col)
	}
	rl.DrawRectangleLines(x, by, w, 22, rl.NewColor(200, 200, 210, 255))

	// mana + meter
	drawThinBar(x, by+26, w, leftAligned, float32(f.Mana/f.MaxMana), rl.NewColor(60, 120, 230, 255))
	drawThinBar(x, by+38, w, leftAligned, float32(f.Meter/100), rl.NewColor(210, 60, 220, 255))
}

func drawThinBar(x, y, w int32, leftAligned bool, frac float32, col rl.Color) {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	rl.DrawRectangle(x, y, w, 8, rl.NewColor(20, 20, 28, 255))
	fw := int32(float32(w) * frac)
	if leftAligned {
		rl.DrawRectangle(x, y, fw, 8, col)
	} else {
		rl.DrawRectangle(x+w-fw, y, fw, 8, col)
	}
}

func drawPips(cx, cy int32, won, need int, leftAligned bool) {
	for i := 0; i < need; i++ {
		off := int32(i*24) + 6
		if !leftAligned {
			off = -off
		}
		c := rl.NewColor(70, 60, 70, 255)
		if i < won {
			c = rl.NewColor(255, 210, 60, 255)
		}
		rl.DrawCircle(cx+off, cy, 8, c)
	}
}

func drawAnnounce(v arena.MatchView) {
	if v.Announce == "" {
		return
	}
	size := int32(64)
	tw := rl.MeasureText(v.Announce, size)
	x := screenW/2 - tw/2
	y := int32(220)
	// shadow + glow
	rl.DrawText(v.Announce, x+3, y+3, size, rl.NewColor(0, 0, 0, 200))
	col := rl.NewColor(255, 60, 40, 255)
	if v.Over {
		col = rl.NewColor(255, 210, 60, 255)
	}
	rl.DrawText(v.Announce, x, y, size, col)
}

// speciesColor derives a vivid, stable RGB from the species name.
func speciesColor(name string) rl.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	hue := float64(h.Sum32()%360) / 360.0
	r, g, b := hsv(hue, 0.7, 0.95)
	return rl.NewColor(r, g, b, 255)
}

func shade(c rl.Color, f float32) rl.Color {
	return rl.NewColor(uint8(float32(c.R)*f), uint8(float32(c.G)*f), uint8(float32(c.B)*f), c.A)
}

// hsv converts h,s,v in [0,1] to 8-bit RGB.
func hsv(h, s, v float64) (uint8, uint8, uint8) {
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}
