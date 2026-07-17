package world

import (
	"math"
	"math/rand"
)

// Movement / animation tuning constants.
const (
	walkSpeed       = 6.0  // cells per second while wandering
	dwellMin        = 2.0  // min seconds to pause at a target
	dwellSpan       = 3.0  // extra random dwell (2–5s total)
	bounceAmp       = 1.0  // vertical bounce amplitude (cells) when working
	bouncePeriod    = 0.3  // seconds
	shakeAmp        = 1.0  // horizontal shake amplitude (cells) on error
	shakePeriod     = 0.12 // seconds
	repulseRange    = 6.0  // columns; below this agents push apart
	repulseSpeed    = 1.0  // cells per second of push
	walkFramePeriod = 0.25 // seconds per walk frame swap
)

// Step advances a single agent by dt seconds within its zone.
func (a *Agent) Step(dt float32, z *Zone, rng *rand.Rand) {
	a.clock += dt
	if z == nil {
		return
	}
	switch a.State {
	case StateIdle:
		a.stepIdle(dt, z, rng)
	case StateThinking:
		// stationary; keep resting position
		a.Pos.X = a.baseX
		a.Pos.Y = a.baseY
	case StateWorking:
		a.Pos.X = a.baseX
		a.Pos.Y = a.baseY - bounce(a.stateAge())
	case StateError:
		a.Pos.Y = a.baseY
		a.Pos.X = a.baseX + shake(a.stateAge())
	case StateDone:
		a.Pos.X = a.baseX
		a.Pos.Y = a.baseY
	}
}

func (a *Agent) stepIdle(dt float32, z *Zone, rng *rand.Rand) {
	spriteW, spriteH := a.spriteSize()
	if a.reachedTarget() {
		if a.dwell > 0 {
			a.dwell -= dt
		} else {
			a.Target = z.randomPoint(rng, spriteW, spriteH)
			a.dwell = dwellMin + rng.Float32()*dwellSpan
		}
		a.baseX, a.baseY = a.Pos.X, a.Pos.Y
		return
	}
	a.lerpTo(a.Target, walkSpeed*dt)
	if s := signf(a.Target.X - a.Pos.X); s != 0 {
		a.Facing = s
	}
	a.baseX, a.baseY = a.Pos.X, a.Pos.Y
}

// spriteSize returns the agent's rendered footprint (subagents are smaller).
func (a *Agent) spriteSize() (int, int) {
	if a.IsSub() {
		return 4, 2
	}
	return a.Species.Idle.W, a.Species.Idle.H
}

func (a *Agent) reachedTarget() bool {
	return absf(a.Target.X-a.Pos.X) < 0.4 && absf(a.Target.Y-a.Pos.Y) < 0.4
}

// lerpTo moves the position toward target by at most maxStep cells.
func (a *Agent) lerpTo(target Vec2, maxStep float32) {
	dx := target.X - a.Pos.X
	dy := target.Y - a.Pos.Y
	dist := float32(math.Hypot(float64(dx), float64(dy)))
	if dist <= maxStep || dist == 0 {
		a.Pos = target
		return
	}
	a.Pos.X += dx / dist * maxStep
	a.Pos.Y += dy / dist * maxStep
}

// WalkFrame returns 0, 1, or 2 (idle/walk-a/walk-b) for the current animation.
func (a *Agent) WalkFrame() int {
	if a.State != StateIdle || a.reachedTarget() {
		return 0
	}
	// alternate walk-a / walk-b
	if int(a.clock/walkFramePeriod)%2 == 0 {
		return 1
	}
	return 2
}

// bounce is a positive vertical offset following |sin|, one bounce per period.
func bounce(t float32) float32 {
	return bounceAmp * float32(math.Abs(math.Sin(float64(t)/bouncePeriod*math.Pi)))
}

// shake returns a ±amplitude square-ish oscillation for the error state.
func shake(t float32) float32 {
	if int(t/shakePeriod)%2 == 0 {
		return shakeAmp
	}
	return -shakeAmp
}
