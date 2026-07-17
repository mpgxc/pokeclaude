package world

import (
	"math"
	"math/rand"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

// Space Drift tuning constants.
const (
	spaceDriftSpeed   = 7.0  // baseline ship cruise speed (cells/s)
	spaceBoostSpeed   = 15.0 // cruise speed while the engine is boosting (Working)
	spaceThinkSpeed   = 3.0  // slow drift while Thinking
	spaceSteer        = 26.0 // steering acceleration (cells/s²)
	spaceDodgeRange   = 12.0 // how far ahead a ship reacts to an obstacle
	spaceFireRange    = 22.0 // laser lock-on range
	spaceFireCooldown = 0.55 // seconds between shots
	projectileSpeed   = 46.0
	projectileLife    = 1.2
	obstacleMin       = 3.5 // slowest obstacle speed
	obstacleSpan      = 5.0 // extra random obstacle speed
	spawnEvery        = 1.1 // seconds between obstacle spawns
	warpDuration      = 0.7 // seconds of warp distortion
	hitDuration       = 0.35
	explosionTTL      = 0.5
	starParallaxBase  = 9.0 // base starfield scroll speed (speed lines)
)

// ObstacleKind distinguishes space hazards.
type ObstacleKind int

const (
	Asteroid ObstacleKind = iota
	Comet
)

// ship is one agent's spacecraft in Space Drift mode.
type ship struct {
	id       AgentID
	species  dex.Species
	nick     string
	isSub    bool
	pos      Vec2
	vel      Vec2
	cooldown float32
	hitTimer float32
	warp     float32 // 1→0 warp animation progress; 0 = not warping
	warpTo   Vec2
	state    State
}

// obstacle is a moving space hazard.
type obstacle struct {
	kind ObstacleKind
	pos  Vec2
	vel  Vec2
	size int // 1 small, 2 large
	hp   int
}

func (o *obstacle) radius() float32 { return 0.6 + float32(o.size)*0.8 }

// projectile is a laser bolt fired by a ship.
type projectile struct {
	pos   Vec2
	vel   Vec2
	owner AgentID
	life  float32
}

// explosion is a short-lived burst effect.
type explosion struct {
	pos Vec2
	age float32
	big bool
}

// star is a background point contributing to the speed-line effect.
type star struct {
	pos   Vec2
	speed float32 // parallax multiplier
}

// SpaceField is the Space Drift sub-simulation: ships, hazards, lasers and the
// scrolling starfield, all within the map bounds.
type SpaceField struct {
	bounds      Rect
	ships       map[AgentID]*ship
	obstacles   []*obstacle
	projectiles []*projectile
	explosions  []*explosion
	stars       []star
	rng         *rand.Rand
	clock       float32
	spawnTimer  float32
	warp        float32 // global warp intensity 1→0
}

// newSpaceField builds an empty field seeded for the given bounds.
func newSpaceField(bounds Rect, rng *rand.Rand) *SpaceField {
	f := &SpaceField{
		bounds: bounds,
		ships:  map[AgentID]*ship{},
		rng:    rng,
	}
	f.seedStars()
	return f
}

// seedStars (re)creates the starfield to match the current bounds.
func (f *SpaceField) seedStars() {
	if f.bounds.W <= 0 || f.bounds.H <= 0 {
		f.stars = nil
		return
	}
	n := (f.bounds.W * f.bounds.H) / 11
	if n < 12 {
		n = 12
	}
	f.stars = make([]star, n)
	for i := range f.stars {
		f.stars[i] = star{
			pos:   f.randomPoint(),
			speed: 0.35 + f.rng.Float32()*1.4,
		}
	}
}

// setBounds updates the field bounds and reseeds the starfield.
func (f *SpaceField) setBounds(b Rect) {
	f.bounds = b
	f.seedStars()
	for _, s := range f.ships {
		s.pos = f.clampInside(s.pos, 0)
	}
}

func (f *SpaceField) randomPoint() Vec2 {
	return Vec2{
		X: float32(f.bounds.X) + f.rng.Float32()*float32(f.bounds.W),
		Y: float32(f.bounds.Y) + f.rng.Float32()*float32(f.bounds.H),
	}
}

// sync creates ships for new agents and drops ships for departed ones, keeping
// each ship's state in step with its agent.
func (f *SpaceField) sync(agents map[AgentID]*Agent) {
	for id, a := range agents {
		s, ok := f.ships[id]
		if !ok {
			ang := f.rng.Float64() * 2 * math.Pi
			s = &ship{
				id:      id,
				species: a.Species,
				nick:    a.Nick,
				isSub:   a.IsSub(),
				pos:     f.randomPoint(),
				vel: Vec2{
					X: float32(math.Cos(ang)) * spaceDriftSpeed,
					Y: float32(math.Sin(ang)) * spaceDriftSpeed * 0.5,
				},
			}
			f.ships[id] = s
		}
		// react to a fresh error with a jolt
		if a.State == StateError && s.state != StateError {
			s.hitTimer = hitDuration
		}
		s.state = a.State
	}
	for id := range f.ships {
		if _, ok := agents[id]; !ok {
			delete(f.ships, id)
		}
	}
}

// triggerWarp starts a warp jump for every ship and the global distortion.
func (f *SpaceField) triggerWarp() {
	f.warp = warpDuration
	for _, s := range f.ships {
		s.warp = warpDuration
		s.warpTo = f.warpTarget(s.pos)
	}
}

// warpTarget picks a destination in a different quadrant than the origin.
func (f *SpaceField) warpTarget(from Vec2) Vec2 {
	halfW, halfH := f.bounds.W/2, f.bounds.H/2
	if halfW < 1 || halfH < 1 {
		return f.randomPoint()
	}
	fromQx := 0
	if int(from.X)-f.bounds.X >= halfW {
		fromQx = 1
	}
	qx := 1 - fromQx
	qy := f.rng.Intn(2)
	x := f.bounds.X + qx*halfW + 1 + f.rng.Intn(maxInt(1, halfW-2))
	y := f.bounds.Y + qy*halfH + 1 + f.rng.Intn(maxInt(1, halfH-2))
	return Vec2{X: float32(x), Y: float32(y)}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// step advances the whole field by dt seconds.
func (f *SpaceField) step(dt float32, agents map[AgentID]*Agent) {
	f.clock += dt
	f.sync(agents)

	if f.warp > 0 {
		f.warp -= dt
		if f.warp < 0 {
			f.warp = 0
		}
	}

	f.stepStars(dt)
	f.stepShips(dt)
	f.stepObstacles(dt)
	f.stepProjectiles(dt)
	f.collide()
	f.stepExplosions(dt)
}

// stepStars scrolls the starfield leftward to create the speed-line effect,
// accelerating while any ship boosts or during a warp.
func (f *SpaceField) stepStars(dt float32) {
	boost := float32(1)
	for _, s := range f.ships {
		if s.state == StateWorking {
			boost = 2.2
			break
		}
	}
	if f.warp > 0 {
		boost += 8 * (f.warp / warpDuration)
	}
	left := float32(f.bounds.X)
	right := float32(f.bounds.Right())
	for i := range f.stars {
		f.stars[i].pos.X -= starParallaxBase * f.stars[i].speed * boost * dt
		if f.stars[i].pos.X < left {
			f.stars[i].pos.X = right
			f.stars[i].pos.Y = float32(f.bounds.Y) + f.rng.Float32()*float32(f.bounds.H)
		}
	}
}

// stepShips updates ship AI (dodge + fire), warp jumps and integration.
func (f *SpaceField) stepShips(dt float32) {
	for _, s := range f.ships {
		if s.hitTimer > 0 {
			s.hitTimer -= dt
		}
		if s.cooldown > 0 {
			s.cooldown -= dt
		}

		// warp jump: snap to destination at the midpoint of the animation
		if s.warp > 0 {
			prev := s.warp
			s.warp -= dt
			if prev >= warpDuration/2 && s.warp < warpDuration/2 {
				s.pos = s.warpTo
			}
			if s.warp < 0 {
				s.warp = 0
			}
		}

		target := f.cruiseSpeed(s.state)
		f.avoid(s, dt)
		f.autoFire(s)

		// ease current speed toward the target cruise speed
		sp := speed(s.vel)
		if sp < 0.01 {
			ang := f.rng.Float64() * 2 * math.Pi
			s.vel = Vec2{X: float32(math.Cos(ang)), Y: float32(math.Sin(ang))}
			sp = 1
		}
		desired := target
		newSp := sp + clampf(desired-sp, -spaceSteer*dt, spaceSteer*dt)
		s.vel.X = s.vel.X / sp * newSp
		s.vel.Y = s.vel.Y / sp * newSp

		s.pos.X += s.vel.X * dt
		s.pos.Y += s.vel.Y * dt
		s.pos = f.wrap(s.pos)
	}
}

func (f *SpaceField) cruiseSpeed(st State) float32 {
	switch st {
	case StateWorking:
		return spaceBoostSpeed
	case StateThinking:
		return spaceThinkSpeed
	default:
		return spaceDriftSpeed
	}
}

// avoid steers a ship away from the nearest obstacle in its path.
func (f *SpaceField) avoid(s *ship, dt float32) {
	var nearest *obstacle
	nd := float32(spaceDodgeRange)
	for _, o := range f.obstacles {
		d := dist(s.pos, o.pos)
		if d < nd {
			nd = d
			nearest = o
		}
	}
	if nearest == nil {
		return
	}
	// push perpendicular to the line toward the obstacle
	away := Vec2{X: s.pos.X - nearest.pos.X, Y: s.pos.Y - nearest.pos.Y}
	al := speed(away)
	if al < 0.01 {
		return
	}
	strength := spaceSteer * (1 - nd/spaceDodgeRange)
	s.vel.X += away.X / al * strength * dt
	s.vel.Y += away.Y / al * strength * dt
}

// autoFire shoots at an obstacle roughly ahead of the ship, if the laser is
// off cooldown.
func (f *SpaceField) autoFire(s *ship) {
	if s.cooldown > 0 || s.warp > 0 {
		return
	}
	sp := speed(s.vel)
	if sp < 0.01 {
		return
	}
	dir := Vec2{X: s.vel.X / sp, Y: s.vel.Y / sp}
	for _, o := range f.obstacles {
		to := Vec2{X: o.pos.X - s.pos.X, Y: o.pos.Y - s.pos.Y}
		d := speed(to)
		if d > spaceFireRange || d < 0.01 {
			continue
		}
		if (to.X*dir.X+to.Y*dir.Y)/d < 0.55 {
			continue // not in front
		}
		f.projectiles = append(f.projectiles, &projectile{
			pos:   Vec2{X: s.pos.X + dir.X, Y: s.pos.Y + dir.Y},
			vel:   Vec2{X: to.X / d * projectileSpeed, Y: to.Y / d * projectileSpeed},
			owner: s.id,
			life:  projectileLife,
		})
		s.cooldown = spaceFireCooldown
		return
	}
}

// stepObstacles spawns new hazards on a timer and moves existing ones, removing
// those that drift well off-screen.
func (f *SpaceField) stepObstacles(dt float32) {
	f.spawnTimer -= dt
	maxObs := (f.bounds.W * f.bounds.H) / 240
	if maxObs < 3 {
		maxObs = 3
	}
	if f.spawnTimer <= 0 && len(f.obstacles) < maxObs {
		f.spawnObstacle()
		f.spawnTimer = spawnEvery
	}
	kept := f.obstacles[:0]
	for _, o := range f.obstacles {
		o.pos.X += o.vel.X * dt
		o.pos.Y += o.vel.Y * dt
		if f.inMargin(o.pos, 6) {
			kept = append(kept, o)
		}
	}
	f.obstacles = kept
}

// spawnObstacle introduces a hazard from a random edge, heading across the field.
func (f *SpaceField) spawnObstacle() {
	b := f.bounds
	var pos, vel Vec2
	edge := f.rng.Intn(4)
	sp := obstacleMin + f.rng.Float32()*obstacleSpan
	switch edge {
	case 0: // from left
		pos = Vec2{X: float32(b.X - 2), Y: float32(b.Y) + f.rng.Float32()*float32(b.H)}
		vel = Vec2{X: sp, Y: (f.rng.Float32() - 0.5) * sp}
	case 1: // from right
		pos = Vec2{X: float32(b.Right() + 2), Y: float32(b.Y) + f.rng.Float32()*float32(b.H)}
		vel = Vec2{X: -sp, Y: (f.rng.Float32() - 0.5) * sp}
	case 2: // from top
		pos = Vec2{X: float32(b.X) + f.rng.Float32()*float32(b.W), Y: float32(b.Y - 2)}
		vel = Vec2{X: (f.rng.Float32() - 0.5) * sp, Y: sp}
	default: // from bottom
		pos = Vec2{X: float32(b.X) + f.rng.Float32()*float32(b.W), Y: float32(b.Bottom() + 2)}
		vel = Vec2{X: (f.rng.Float32() - 0.5) * sp, Y: -sp}
	}
	kind := Asteroid
	size := 1 + f.rng.Intn(2)
	hp := size
	if f.rng.Intn(3) == 0 {
		kind = Comet
		size = 1
		hp = 1
		// comets are faster
		vel.X *= 1.6
		vel.Y *= 1.6
	}
	f.obstacles = append(f.obstacles, &obstacle{kind: kind, pos: pos, vel: vel, size: size, hp: hp})
}

// stepProjectiles moves lasers and expires them.
func (f *SpaceField) stepProjectiles(dt float32) {
	kept := f.projectiles[:0]
	for _, p := range f.projectiles {
		p.pos.X += p.vel.X * dt
		p.pos.Y += p.vel.Y * dt
		p.life -= dt
		if p.life > 0 && f.inMargin(p.pos, 2) {
			kept = append(kept, p)
		}
	}
	f.projectiles = kept
}

// collide resolves laser↔hazard and ship↔hazard collisions.
func (f *SpaceField) collide() {
	// lasers vs obstacles
	liveP := f.projectiles[:0]
	for _, p := range f.projectiles {
		hit := false
		for _, o := range f.obstacles {
			if o.hp <= 0 {
				continue
			}
			if dist(p.pos, o.pos) <= o.radius()+0.5 {
				o.hp--
				hit = true
				f.explosions = append(f.explosions, &explosion{pos: p.pos, big: o.hp <= 0})
				break
			}
		}
		if !hit {
			liveP = append(liveP, p)
		}
	}
	f.projectiles = liveP

	// ships vs obstacles
	for _, s := range f.ships {
		if s.warp > 0 {
			continue // intangible mid-jump
		}
		for _, o := range f.obstacles {
			if o.hp <= 0 {
				continue
			}
			if dist(s.pos, o.pos) <= o.radius()+1.0 {
				o.hp = 0
				s.hitTimer = hitDuration
				// knockback
				kb := Vec2{X: s.pos.X - o.pos.X, Y: s.pos.Y - o.pos.Y}
				if l := speed(kb); l > 0.01 {
					s.vel.X += kb.X / l * spaceDriftSpeed
					s.vel.Y += kb.Y / l * spaceDriftSpeed
				}
				f.explosions = append(f.explosions, &explosion{pos: o.pos, big: true})
			}
		}
	}

	// drop destroyed obstacles
	liveO := f.obstacles[:0]
	for _, o := range f.obstacles {
		if o.hp > 0 {
			liveO = append(liveO, o)
		}
	}
	f.obstacles = liveO
}

// stepExplosions ages burst effects and removes finished ones.
func (f *SpaceField) stepExplosions(dt float32) {
	kept := f.explosions[:0]
	for _, e := range f.explosions {
		e.age += dt
		if e.age < explosionTTL {
			kept = append(kept, e)
		}
	}
	f.explosions = kept
}

// wrap keeps a position toroidally inside the bounds.
func (f *SpaceField) wrap(p Vec2) Vec2 {
	w, h := float32(f.bounds.W), float32(f.bounds.H)
	if w <= 0 || h <= 0 {
		return p
	}
	x0, y0 := float32(f.bounds.X), float32(f.bounds.Y)
	if p.X < x0 {
		p.X += w
	} else if p.X >= x0+w {
		p.X -= w
	}
	if p.Y < y0 {
		p.Y += h
	} else if p.Y >= y0+h {
		p.Y -= h
	}
	return p
}

// clampInside keeps a position within the bounds (with padding), non-toroidal.
func (f *SpaceField) clampInside(p Vec2, pad float32) Vec2 {
	p.X = clampf(p.X, float32(f.bounds.X)+pad, float32(f.bounds.Right())-1-pad)
	p.Y = clampf(p.Y, float32(f.bounds.Y)+pad, float32(f.bounds.Bottom())-1-pad)
	return p
}

// inMargin reports whether p is within margin cells of the bounds.
func (f *SpaceField) inMargin(p Vec2, margin float32) bool {
	return p.X >= float32(f.bounds.X)-margin && p.X <= float32(f.bounds.Right())+margin &&
		p.Y >= float32(f.bounds.Y)-margin && p.Y <= float32(f.bounds.Bottom())+margin
}

func speed(v Vec2) float32 { return float32(math.Hypot(float64(v.X), float64(v.Y))) }
func dist(a, b Vec2) float32 {
	return float32(math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)))
}
