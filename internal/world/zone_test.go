package world

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

func TestLayoutZones(t *testing.T) {
	bounds := Rect{X: 1, Y: 3, W: 80, H: 20}

	t.Run("single", func(t *testing.T) {
		zs := []*Zone{{ID: "a"}}
		layoutZones(zs, bounds)
		if zs[0].Rect != bounds {
			t.Errorf("single zone = %+v, want %+v", zs[0].Rect, bounds)
		}
	})

	t.Run("two_columns", func(t *testing.T) {
		zs := []*Zone{{ID: "a"}, {ID: "b"}}
		layoutZones(zs, bounds)
		if zs[0].Rect.W+zs[1].Rect.W != bounds.W {
			t.Errorf("columns don't tile width: %d + %d != %d", zs[0].Rect.W, zs[1].Rect.W, bounds.W)
		}
		if zs[0].Rect.H != bounds.H || zs[1].Rect.H != bounds.H {
			t.Error("columns should be full height")
		}
	})

	t.Run("four_grid", func(t *testing.T) {
		zs := []*Zone{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
		layoutZones(zs, bounds)
		// no zone should escape bounds
		for _, z := range zs {
			if z.Rect.X < bounds.X || z.Rect.Right() > bounds.Right() ||
				z.Rect.Y < bounds.Y || z.Rect.Bottom() > bounds.Bottom() {
				t.Errorf("zone %s escapes bounds: %+v", z.ID, z.Rect)
			}
		}
	})
}

func TestLayoutDeterministic(t *testing.T) {
	bounds := Rect{X: 1, Y: 3, W: 80, H: 20}
	mk := func() []Rect {
		zs := []*Zone{{ID: "a", CWD: "/a"}, {ID: "b", CWD: "/b"}, {ID: "c", CWD: "/c"}}
		for i, z := range zs {
			z.seed = int64(i + 1)
		}
		layoutZones(zs, bounds)
		return []Rect{zs[0].Rect, zs[1].Rect, zs[2].Rect}
	}
	a, b := mk(), mk()
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("layout not deterministic at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestSlugStable(t *testing.T) {
	if slug("/Home/Dev/Proj X") != slug("/Home/Dev/Proj X") {
		t.Error("slug not stable")
	}
	if slug("/a/b") == slug("/a/c") {
		t.Error("distinct cwds should slug differently")
	}
}

func TestZoneKindStable(t *testing.T) {
	z1 := newZone("/home/dev/proj", 0, time.Unix(0, 0))
	z2 := newZone("/home/dev/proj", 5, time.Unix(100, 0))
	if z1.Kind != z2.Kind {
		t.Error("same cwd should map to same kind")
	}
}

func TestFromTool(t *testing.T) {
	cases := []struct {
		tool  string
		input string
		want  string
	}{
		{"Read", `{"file_path":"/a/b/world.go"}`, "🔍 lendo world.go"},
		{"Bash", `{"command":"go test ./..."}`, "⚡ go test ./..."},
		{"Edit", `{"file_path":"/x/consumer.go"}`, "✏ editando consumer.go"},
		{"Task", `{}`, "🥚 chamando ajuda"},
		{"WebFetch", `{"url":"https://pkg.go.dev/net/http"}`, "🌐 pkg.go.dev"},
		{"TodoWrite", `{}`, "📋 planejando"},
		{"Frobnicate", `{}`, "⚙ Frobnicate"},
	}
	for _, c := range cases {
		got := FromTool(c.tool, json.RawMessage(c.input))
		full := string(got.Icon) + " " + got.Text
		if full != c.want {
			t.Errorf("FromTool(%s) = %q, want %q", c.tool, full, c.want)
		}
	}
}

func TestSanitizeTruncates(t *testing.T) {
	long := "this is a very long command that certainly exceeds the bubble width limit"
	got := sanitize(long)
	if width := runewidth.StringWidth(got); width > maxThoughtWidth {
		t.Errorf("sanitized width = %d, want <= %d (%q)", width, maxThoughtWidth, got)
	}
}

func TestSanitizeStripsControl(t *testing.T) {
	got := sanitize("hello\n\tworld\x1b[31m!")
	if got != "hello world!" {
		t.Errorf("sanitize = %q, want %q", got, "hello world!")
	}
}
