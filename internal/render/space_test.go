package render

import (
	"strings"
	"testing"
	"time"

	"github.com/mpgxc/pokeclaude/internal/dex"
	"github.com/mpgxc/pokeclaude/internal/world"
)

func spaceSampleView(now time.Time, warp float32) world.View {
	sp := dex.All()[0]
	b := world.Rect{X: 1, Y: 3, W: 78, H: 14}
	return world.View{
		Bounds:    b,
		ZoneTotal: 2,
		PageCount: 1,
		Now:       now,
		Mode:      world.ModeSpaceDrift,
		Agents: []world.AgentView{
			{ID: "s1", Species: sp, Nick: sp.Name, State: world.StateWorking, LastSeen: now},
		},
		Space: world.SpaceView{
			Bounds: b,
			Warp:   warp,
			Ships: []world.ShipView{
				{ID: "s1", Species: sp, Nick: sp.Name, Pos: world.Vec2{X: 20, Y: 8}, Vel: world.Vec2{X: 6}, State: world.StateWorking, Boosting: true},
				{ID: "s2", Species: sp, Nick: "sub", IsSub: true, Pos: world.Vec2{X: 40, Y: 10}, Vel: world.Vec2{X: -4}},
			},
			Obstacles: []world.ObstacleView{
				{Kind: world.Asteroid, Pos: world.Vec2{X: 30, Y: 6}, Size: 2, Vel: world.Vec2{X: -4}},
				{Kind: world.Comet, Pos: world.Vec2{X: 50, Y: 9}, Size: 1, Vel: world.Vec2{X: -8, Y: 1}},
			},
			Projectiles: []world.ProjectileView{{Pos: world.Vec2{X: 25, Y: 8}, Vel: world.Vec2{X: 40}}},
			Explosions:  []world.ExplosionView{{Pos: world.Vec2{X: 35, Y: 7}, Frame: 0, Big: true}},
			Stars: []world.StarView{
				{Pos: world.Vec2{X: 10, Y: 5}, Speed: 1.4},
				{Pos: world.Vec2{X: 60, Y: 12}, Speed: 0.5},
			},
		},
	}
}

func TestRenderSpaceModeDimensions(t *testing.T) {
	out := RenderFrame(spaceSampleView(time.Unix(1_700_000_000, 0), 0), 80, 24)
	if lines := strings.Count(out, "\n") + 1; lines != 24 {
		t.Errorf("space frame has %d lines, want 24", lines)
	}
	if !strings.Contains(out, "Space Drift") {
		t.Error("header should show the Space Drift mode")
	}
}

func TestRenderSpaceShowsHyperHint(t *testing.T) {
	// a roomy terminal has space for the controls hint
	out := RenderFrame(spaceSampleView(time.Unix(1_700_000_000, 0), 0), 110, 30)
	if !strings.Contains(out, "[h] hiper") {
		t.Error("header should show the hyper-speed hint in space mode")
	}
}

func TestRenderSpaceWarpNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic rendering warp: %v", r)
		}
	}()
	// full warp intensity with entities near the edges
	RenderFrame(spaceSampleView(time.Unix(0, 0), 1.0), 80, 24)
}

func TestHullCellsDirection(t *testing.T) {
	cases := []struct {
		vel  world.Vec2
		want shipDir
	}{
		{world.Vec2{X: 5}, dirRight},
		{world.Vec2{X: -5}, dirLeft},
		{world.Vec2{Y: -5}, dirUp},
		{world.Vec2{Y: 5}, dirDown},
	}
	for _, c := range cases {
		if _, _, _, dir := hullCells(c.vel); dir != c.want {
			t.Errorf("hullCells(%+v) dir = %v, want %v", c.vel, dir, c.want)
		}
	}
}
