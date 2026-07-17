package world

import (
	"hash/fnv"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ZoneKind is a purely decorative biome derived from the cwd hash.
type ZoneKind int

const (
	ZoneGrass ZoneKind = iota
	ZoneLake
	ZoneCave
)

// Deco is a single decorative glyph placed inside a zone at spawn time.
type Deco struct {
	X, Y  int
	Rune  rune
	Color string
}

// Zone is a bounded area on the map, one per distinct cwd.
type Zone struct {
	ID         string // slug of the cwd
	Label      string // filepath.Base(cwd)
	CWD        string
	Rect       Rect
	Kind       ZoneKind
	Deco       []Deco
	CreatedIdx int       // creation order, for stable layout
	LastActive time.Time // most recent event, for "most active" paging
	seed       int64
	emptyAt    time.Time // when the zone became empty (for GC); zero if occupied
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns a cwd into a stable identifier.
func slug(cwd string) string {
	s := strings.ToLower(cwd)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// hashCWD returns a stable hash of the cwd, used for kind + deco seeding.
func hashCWD(cwd string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(cwd))
	return h.Sum32()
}

// newZone creates a zone for a cwd. Deco is generated later, once the zone has
// a Rect (see generateDeco), because it depends on the zone size.
func newZone(cwd string, idx int, now time.Time) *Zone {
	h := hashCWD(cwd)
	label := filepath.Base(cwd)
	if label == "" || label == "." || label == "/" {
		label = "root"
	}
	return &Zone{
		ID:         slug(cwd),
		Label:      label,
		CWD:        cwd,
		Kind:       ZoneKind(h % 3),
		CreatedIdx: idx,
		LastActive: now,
		seed:       int64(h),
	}
}

// generateDeco (re)builds the decorative glyphs for the zone's current Rect.
// It is seeded from the cwd hash so the layout is stable across reflows.
func (z *Zone) generateDeco() {
	z.Deco = nil
	if z.Rect.W < 4 || z.Rect.H < 4 {
		return
	}
	rng := rand.New(rand.NewSource(z.seed))
	// interior area (leave a 1-cell border for the frame)
	ix, iy := z.Rect.X+1, z.Rect.Y+1
	iw, ih := z.Rect.W-2, z.Rect.H-2
	if iw <= 0 || ih <= 0 {
		return
	}
	switch z.Kind {
	case ZoneGrass:
		n := (iw * ih) / 18
		for i := 0; i < n; i++ {
			z.Deco = append(z.Deco, Deco{
				X: ix + rng.Intn(iw), Y: iy + rng.Intn(ih),
				Rune: '"', Color: "34",
			})
		}
	case ZoneLake:
		// a lake band near the bottom
		lakeY := iy + ih - 1
		for x := 0; x < iw; x++ {
			z.Deco = append(z.Deco, Deco{X: ix + x, Y: lakeY, Rune: '~', Color: "45"})
		}
	case ZoneCave:
		n := (iw * ih) / 22
		rocks := []rune{'^', '*'}
		for i := 0; i < n; i++ {
			z.Deco = append(z.Deco, Deco{
				X: ix + rng.Intn(iw), Y: iy + rng.Intn(ih),
				Rune: rocks[rng.Intn(len(rocks))], Color: "244",
			})
		}
	}
}

// randomPoint returns a random cell inside the zone's usable interior (with a
// 1-cell padding for the border), as float coordinates.
func (z *Zone) randomPoint(rng *rand.Rand, spriteW, spriteH int) Vec2 {
	pad := 1
	minX := z.Rect.X + pad
	maxX := z.Rect.Right() - pad - spriteW
	minY := z.Rect.Y + pad
	maxY := z.Rect.Bottom() - pad - spriteH
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	x := minX
	if maxX > minX {
		x = minX + rng.Intn(maxX-minX+1)
	}
	y := minY
	if maxY > minY {
		y = minY + rng.Intn(maxY-minY+1)
	}
	return Vec2{X: float32(x), Y: float32(y)}
}

// layoutZones assigns a Rect to each zone in zs, packed into bounds. It handles
// 1, 2, 3–4 zones directly; callers should pass at most 4 zones (one page).
func layoutZones(zs []*Zone, bounds Rect) {
	n := len(zs)
	switch {
	case n == 0:
		return
	case n == 1:
		zs[0].Rect = bounds
	case n == 2:
		// two columns side by side
		w := bounds.W / 2
		zs[0].Rect = Rect{bounds.X, bounds.Y, w, bounds.H}
		zs[1].Rect = Rect{bounds.X + w, bounds.Y, bounds.W - w, bounds.H}
	default: // 3 or 4 => 2x2 grid
		w := bounds.W / 2
		h := bounds.H / 2
		rects := []Rect{
			{bounds.X, bounds.Y, w, h},
			{bounds.X + w, bounds.Y, bounds.W - w, h},
			{bounds.X, bounds.Y + h, w, bounds.H - h},
			{bounds.X + w, bounds.Y + h, bounds.W - w, bounds.H - h},
		}
		for i, z := range zs {
			z.Rect = rects[i]
		}
	}
	for _, z := range zs {
		z.generateDeco()
	}
}
