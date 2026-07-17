package world

import (
	"math/rand"
	"testing"
	"time"

	"github.com/mpgxc/pokeclaude/internal/protocol"
)

func spaceBounds() Rect { return Rect{X: 1, Y: 3, W: 80, H: 16} }

func TestModeCycle(t *testing.T) {
	w := newTestWorld()
	if w.Mode() != ModeZone {
		t.Fatalf("default mode = %v, want Zone", w.Mode())
	}
	w.CycleMode()
	if w.Mode() != ModeSpaceDrift {
		t.Errorf("after 1 cycle = %v, want Space Drift", w.Mode())
	}
	w.CycleMode()
	if w.Mode() != ModeZone {
		t.Errorf("after 2 cycles = %v, want Zone", w.Mode())
	}
}

func TestSpaceSyncTracksAgents(t *testing.T) {
	w := newTestWorld()
	w.Apply(env(protocol.HookSessionStart, "a", "/p1", nil))
	w.Apply(env(protocol.HookSessionStart, "b", "/p2", nil))
	w.SetMode(ModeSpaceDrift)
	w.Tick(0.05, time.Unix(1000, 0))

	sv := w.Snapshot(time.Unix(1000, 0)).Space
	if len(sv.Ships) != 2 {
		t.Errorf("ships = %d, want 2", len(sv.Ships))
	}
	if len(sv.Stars) == 0 {
		t.Error("starfield not seeded")
	}
}

func TestSpaceShipsStayInBounds(t *testing.T) {
	w := newTestWorld()
	for _, id := range []string{"a", "b", "c"} {
		w.Apply(env(protocol.HookSessionStart, id, "/"+id, nil))
	}
	w.SetMode(ModeSpaceDrift)
	now := time.Unix(1000, 0)
	for i := 0; i < 400; i++ {
		now = now.Add(50 * time.Millisecond)
		w.Tick(0.05, now)
	}
	b := spaceBoundsFrom(w)
	for _, s := range w.Snapshot(now).Space.Ships {
		if s.Pos.X < float32(b.X) || s.Pos.X > float32(b.Right()) ||
			s.Pos.Y < float32(b.Y) || s.Pos.Y > float32(b.Bottom()) {
			t.Errorf("ship %s escaped bounds: %+v (bounds %+v)", s.ID, s.Pos, b)
		}
	}
}

func spaceBoundsFrom(w *World) Rect {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.space.bounds
}

func TestWarpTeleportsShips(t *testing.T) {
	w := newTestWorld()
	w.Apply(env(protocol.HookSessionStart, "a", "/p", nil))
	w.SetMode(ModeSpaceDrift)
	now := time.Unix(1000, 0)
	w.Tick(0.05, now)

	before := w.Snapshot(now).Space.Ships[0].Pos
	w.TriggerWarp()
	for i := 0; i < 16; i++ { // 0.8s > warpDuration
		now = now.Add(50 * time.Millisecond)
		w.Tick(0.05, now)
	}
	after := w.Snapshot(now).Space.Ships[0].Pos
	if dist(before, after) < 3 {
		t.Errorf("warp did not move the ship far enough: %v -> %v", before, after)
	}
	b := spaceBoundsFrom(w)
	if after.X < float32(b.X) || after.X > float32(b.Right()) {
		t.Errorf("ship out of bounds after warp: %+v", after)
	}
}

func TestLaserDestroysObstacle(t *testing.T) {
	f := newSpaceField(spaceBounds(), rand.New(rand.NewSource(1)))
	o := &obstacle{kind: Asteroid, pos: Vec2{X: 20, Y: 10}, size: 1, hp: 1}
	f.obstacles = []*obstacle{o}
	f.projectiles = []*projectile{{pos: Vec2{X: 20, Y: 10}, vel: Vec2{X: projectileSpeed}, life: 1}}

	f.collide()

	if len(f.obstacles) != 0 {
		t.Errorf("obstacle survived a direct hit: %d left", len(f.obstacles))
	}
	if len(f.explosions) == 0 {
		t.Error("no explosion spawned on destruction")
	}
	if len(f.projectiles) != 0 {
		t.Error("projectile should be consumed on hit")
	}
}

func TestShipObstacleCollisionDamages(t *testing.T) {
	f := newSpaceField(spaceBounds(), rand.New(rand.NewSource(1)))
	s := &ship{id: "a", pos: Vec2{X: 20, Y: 10}, vel: Vec2{X: 5}}
	f.ships["a"] = s
	f.obstacles = []*obstacle{{kind: Asteroid, pos: Vec2{X: 20, Y: 10}, size: 1, hp: 1}}

	f.collide()

	if s.hitTimer <= 0 {
		t.Error("ship should be flagged as hit after colliding with an asteroid")
	}
	if len(f.obstacles) != 0 {
		t.Error("asteroid should be destroyed on impact")
	}
}

func TestObstacleSpawnRespectsCap(t *testing.T) {
	f := newSpaceField(spaceBounds(), rand.New(rand.NewSource(2)))
	maxObs := (f.bounds.W * f.bounds.H) / 240
	if maxObs < 3 {
		maxObs = 3
	}
	for i := 0; i < 2000; i++ {
		f.stepObstacles(0.05)
	}
	if len(f.obstacles) > maxObs {
		t.Errorf("obstacle count %d exceeds cap %d", len(f.obstacles), maxObs)
	}
}

func TestAutoFireProducesProjectile(t *testing.T) {
	f := newSpaceField(spaceBounds(), rand.New(rand.NewSource(3)))
	s := &ship{id: "a", pos: Vec2{X: 10, Y: 10}, vel: Vec2{X: 6}} // heading right
	f.ships["a"] = s
	// obstacle directly ahead, within range
	f.obstacles = []*obstacle{{kind: Asteroid, pos: Vec2{X: 20, Y: 10}, size: 1, hp: 1}}
	f.autoFire(s)
	if len(f.projectiles) != 1 {
		t.Fatalf("expected 1 projectile, got %d", len(f.projectiles))
	}
	if f.projectiles[0].vel.X <= 0 {
		t.Error("projectile should travel toward the obstacle (positive X)")
	}
}
