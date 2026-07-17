package world

import "github.com/mpgxc/pokeclaude/internal/dex"

// Mode selects how agents move and are rendered.
type Mode int

const (
	ModeZone       Mode = iota // Pokémon walking around cwd zones (default)
	ModeSpaceDrift             // Pokémon piloting spaceships through the void
	modeCount
)

// String returns a short label for the mode.
func (m Mode) String() string {
	switch m {
	case ModeZone:
		return "Zonas"
	case ModeSpaceDrift:
		return "Space Drift"
	default:
		return "?"
	}
}

// Mode returns the current movement mode.
func (w *World) Mode() Mode {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.mode
}

// CycleMode advances to the next movement mode, wrapping around.
func (w *World) CycleMode() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mode = (w.mode + 1) % modeCount
	if w.mode == ModeSpaceDrift {
		// resync ships with the current agents so they appear immediately
		w.space.setBounds(w.bounds)
		w.space.sync(w.agents)
	}
}

// SetMode selects a specific movement mode (used by tests and flags).
func (w *World) SetMode(m Mode) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m < 0 || m >= modeCount {
		return
	}
	w.mode = m
	if m == ModeSpaceDrift {
		w.space.setBounds(w.bounds)
		w.space.sync(w.agents)
	}
}

// TriggerWarp fires the Space Drift hyper-speed jump. It is a no-op outside
// Space Drift mode.
func (w *World) TriggerWarp() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mode == ModeSpaceDrift {
		w.space.triggerWarp()
	}
}

// --- Space Drift render snapshot (value copies) ---

// ShipView is an immutable snapshot of a ship.
type ShipView struct {
	ID       AgentID
	Species  dex.Species
	Nick     string
	IsSub    bool
	Pos      Vec2
	Vel      Vec2
	State    State
	Boosting bool
	Hit      bool
	Warp     float32 // 0 = normal, →1 mid-jump
}

// ObstacleView is an immutable snapshot of a hazard.
type ObstacleView struct {
	Kind ObstacleKind
	Pos  Vec2
	Vel  Vec2
	Size int
}

// ProjectileView is an immutable snapshot of a laser bolt.
type ProjectileView struct {
	Pos Vec2
	Vel Vec2
}

// ExplosionView is an immutable snapshot of a burst effect.
type ExplosionView struct {
	Pos   Vec2
	Frame int // 0..2
	Big   bool
}

// StarView is an immutable snapshot of a background star.
type StarView struct {
	Pos   Vec2
	Speed float32
}

// SpaceView is the full Space Drift render snapshot.
type SpaceView struct {
	Bounds      Rect
	Ships       []ShipView
	Obstacles   []ObstacleView
	Projectiles []ProjectileView
	Explosions  []ExplosionView
	Stars       []StarView
	Warp        float32 // global warp intensity 0..1
}

// snapshot builds a render-facing copy of the field.
func (f *SpaceField) snapshot() SpaceView {
	sv := SpaceView{Bounds: f.bounds, Warp: f.warp / warpDuration}
	for _, s := range f.ships {
		sv.Ships = append(sv.Ships, ShipView{
			ID: s.id, Species: s.species, Nick: s.nick, IsSub: s.isSub,
			Pos: s.pos, Vel: s.vel, State: s.state,
			Boosting: s.state == StateWorking,
			Hit:      s.hitTimer > 0,
			Warp:     s.warp / warpDuration,
		})
	}
	for _, o := range f.obstacles {
		sv.Obstacles = append(sv.Obstacles, ObstacleView{Kind: o.kind, Pos: o.pos, Vel: o.vel, Size: o.size})
	}
	for _, p := range f.projectiles {
		sv.Projectiles = append(sv.Projectiles, ProjectileView{Pos: p.pos, Vel: p.vel})
	}
	for _, e := range f.explosions {
		frame := int(e.age / (explosionTTL / 3))
		if frame > 2 {
			frame = 2
		}
		sv.Explosions = append(sv.Explosions, ExplosionView{Pos: e.pos, Frame: frame, Big: e.big})
	}
	for _, s := range f.stars {
		sv.Stars = append(sv.Stars, StarView{Pos: s.pos, Speed: s.speed})
	}
	return sv
}
