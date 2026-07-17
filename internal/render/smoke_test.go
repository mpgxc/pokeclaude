package render

import (
	"strings"
	"testing"
	"time"

	"github.com/mpgxc/pokeclaude/internal/dex"
	"github.com/mpgxc/pokeclaude/internal/world"
)

func sampleView(now time.Time) world.View {
	sp := dex.All()[0]
	return world.View{
		Bounds:    world.Rect{X: 1, Y: 3, W: 78, H: 14},
		ZoneTotal: 1,
		PageCount: 1,
		Now:       now,
		Zones: []world.ZoneView{{
			ID: "proj", Label: "proj",
			Rect: world.Rect{X: 1, Y: 3, W: 78, H: 14},
			Kind: world.ZoneGrass,
		}},
		Agents: []world.AgentView{{
			ID: "s1", Species: sp, Nick: sp.Name, ZoneID: "proj",
			Pos: world.Vec2{X: 20, Y: 8}, Facing: 1,
			State:    world.StateWorking,
			Thought:  world.Thought{Icon: '⚡', Text: "go test"},
			LastSeen: now,
		}},
	}
}

func TestRenderFrameFullDimensions(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	out := RenderFrame(sampleView(now), 80, 24)
	if lines := strings.Count(out, "\n") + 1; lines != 24 {
		t.Errorf("full frame has %d lines, want 24", lines)
	}
}

func TestRenderFrameCompactFallback(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	out := RenderFrame(sampleView(now), 60, 20)
	if lines := strings.Count(out, "\n") + 1; lines != 20 {
		t.Errorf("compact frame has %d lines, want 20", lines)
	}
	if !strings.Contains(out, "PokéClaude") {
		t.Error("compact frame missing header")
	}
}

func TestRenderNoPanicTinyTerminal(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on tiny terminal: %v", r)
		}
	}()
	RenderFrame(sampleView(time.Unix(0, 0)), 10, 5)
}
