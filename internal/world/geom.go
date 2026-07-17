package world

// Vec2 is a floating-point 2D point in terminal-cell space. Floats let agents
// lerp smoothly between integer cells.
type Vec2 struct {
	X, Y float32
}

// Rect is an axis-aligned rectangle in terminal cells. X,Y is the top-left
// corner; W,H are width and height.
type Rect struct {
	X, Y, W, H int
}

// Contains reports whether the integer cell (x,y) is inside the rect.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Right returns the exclusive right edge (X+W).
func (r Rect) Right() int { return r.X + r.W }

// Bottom returns the exclusive bottom edge (Y+H).
func (r Rect) Bottom() int { return r.Y + r.H }

func absf(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

func signf(f float32) int {
	switch {
	case f > 0.001:
		return 1
	case f < -0.001:
		return -1
	default:
		return 0
	}
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
