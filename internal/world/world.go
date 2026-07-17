package world

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

// GC / lifetime constants.
const (
	zonesPerPage   = 4
	doneFade       = 10 * time.Second
	subagentFade   = 2 * time.Second
	errorDuration  = 2 * time.Second
	greetDuration  = 3 * time.Second
	emptyZoneGrace = 30 * time.Second
	deadSessionTTL = 15 * time.Minute
)

// World holds all live zones and agents plus layout state. It is safe for
// concurrent use, though in practice all mutation happens on the TUI goroutine.
type World struct {
	mu          sync.RWMutex
	zones       map[string]*Zone
	agents      map[AgentID]*Agent
	order       []AgentID // stable creation order for the HUD
	bounds      Rect
	page        int
	nextZoneIdx int
	rng         *rand.Rand
	mode        Mode
	space       *SpaceField
}

// New creates an empty world with the given map bounds, seeded from the clock.
func New(bounds Rect) *World {
	return NewWithSeed(bounds, time.Now().UnixNano())
}

// NewWithSeed is New with an explicit RNG seed, for deterministic tests.
func NewWithSeed(bounds Rect, seed int64) *World {
	rng := rand.New(rand.NewSource(seed))
	return &World{
		zones:  map[string]*Zone{},
		agents: map[AgentID]*Agent{},
		bounds: bounds,
		rng:    rng,
		space:  newSpaceField(bounds, rng),
	}
}

// Resize updates the map bounds and relays out zones and agents.
func (w *World) Resize(bounds Rect) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bounds = bounds
	w.relayout()
	w.space.setBounds(bounds)
}

// Bounds returns the current map bounds.
func (w *World) Bounds() Rect {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.bounds
}

// PageCount returns the number of zone pages.
func (w *World) PageCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	n := len(w.zones)
	if n == 0 {
		return 1
	}
	return (n + zonesPerPage - 1) / zonesPerPage
}

// NextPage advances to the next zone page (wrapping) and relays out.
func (w *World) NextPage() {
	w.mu.Lock()
	defer w.mu.Unlock()
	pc := 1
	if n := len(w.zones); n > 0 {
		pc = (n + zonesPerPage - 1) / zonesPerPage
	}
	w.page = (w.page + 1) % pc
	w.relayout()
}

// Compact reports whether the terminal is too small for the map view.
func (w *World) Compact() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.bounds.W < 80 || w.bounds.H < 24
}

// Tick advances the whole world by dt seconds and performs GC.
func (w *World) Tick(dt float32, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// state timeouts
	for _, a := range w.agents {
		w.applyTimeouts(a, now)
	}

	// movement depends on the active mode
	switch w.mode {
	case ModeSpaceDrift:
		w.space.step(dt, w.agents)
	default:
		for _, a := range w.agents {
			z := w.zones[a.ZoneID]
			a.Step(dt, z, w.rng)
		}
		// soft repulsion between agents sharing a zone
		w.repel(dt)
	}

	// removals (fade timers, dead sessions)
	w.gc(now)
}

// applyTimeouts handles automatic state transitions driven by elapsed time.
func (w *World) applyTimeouts(a *Agent, now time.Time) {
	switch a.State {
	case StateError:
		if a.stateAge() >= float32(errorDuration.Seconds()) {
			a.setState(StateThinking)
		}
	}
	// clear an expired greeting/temp thought
	if !a.Thought.Expires.IsZero() && now.After(a.Thought.Expires) {
		if a.State == StateIdle {
			a.Thought = Thought{}
		}
	}
}

// repel nudges agents apart when they crowd within repulseRange columns.
func (w *World) repel(dt float32) {
	byZone := map[string][]*Agent{}
	for _, a := range w.agents {
		byZone[a.ZoneID] = append(byZone[a.ZoneID], a)
	}
	for zid, list := range byZone {
		z := w.zones[zid]
		if z == nil {
			continue
		}
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				a, b := list[i], list[j]
				dx := a.Pos.X - b.Pos.X
				if absf(dx) < repulseRange && a.State == StateIdle && b.State == StateIdle {
					push := repulseSpeed * dt
					if dx >= 0 {
						a.Pos.X += push
						b.Pos.X -= push
					} else {
						a.Pos.X -= push
						b.Pos.X += push
					}
					clampAgentToZone(a, z)
					clampAgentToZone(b, z)
					a.baseX, b.baseX = a.Pos.X, b.Pos.X
				}
			}
		}
	}
}

// gc removes faded/dead agents and empty zones.
func (w *World) gc(now time.Time) {
	for id, a := range w.agents {
		dead := now.Sub(a.LastSeen) > deadSessionTTL
		faded := !a.fadeAt.IsZero() && !now.Before(a.fadeAt)
		if dead || faded {
			w.removeAgent(id)
		}
	}
	// empty-zone reclamation
	for id, z := range w.zones {
		if w.zoneHasAgents(id) {
			z.emptyAt = time.Time{}
			continue
		}
		if z.emptyAt.IsZero() {
			z.emptyAt = now
		} else if now.Sub(z.emptyAt) > emptyZoneGrace {
			delete(w.zones, id)
		}
	}
	w.relayout()
}

func (w *World) zoneHasAgents(zoneID string) bool {
	for _, a := range w.agents {
		if a.ZoneID == zoneID {
			return true
		}
	}
	return false
}

// removeAgent deletes an agent and any of its subagents from the world.
func (w *World) removeAgent(id AgentID) {
	a := w.agents[id]
	if a == nil {
		return
	}
	// remove children if this is a root
	if !a.IsSub() {
		for cid, c := range w.agents {
			if c.ParentID == id {
				delete(w.agents, cid)
			}
		}
	}
	delete(w.agents, id)
	// prune order slice
	out := w.order[:0]
	for _, oid := range w.order {
		if _, ok := w.agents[oid]; ok {
			out = append(out, oid)
		}
	}
	w.order = out
}

// visibleZonesLocked returns the zones on the current page, most-active first.
func (w *World) visibleZonesLocked() []*Zone {
	all := make([]*Zone, 0, len(w.zones))
	for _, z := range w.zones {
		all = append(all, z)
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].LastActive.Equal(all[j].LastActive) {
			return all[i].LastActive.After(all[j].LastActive)
		}
		return all[i].CreatedIdx < all[j].CreatedIdx
	})
	start := w.page * zonesPerPage
	if start >= len(all) {
		w.page = 0
		start = 0
	}
	end := start + zonesPerPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

// relayout recomputes zone rects for the current page and repositions agents.
func (w *World) relayout() {
	visible := w.visibleZonesLocked()
	prev := make(map[string]Rect, len(visible))
	for _, z := range visible {
		prev[z.ID] = z.Rect
	}
	// clear all rects, then assign visible ones
	for _, z := range w.zones {
		z.Rect = Rect{}
	}
	layoutZones(visible, w.bounds)
	for _, z := range visible {
		old := prev[z.ID]
		if old.W > 0 && old.H > 0 && old != z.Rect {
			for _, a := range w.agents {
				if a.ZoneID == z.ID {
					remapAgent(a, old, z.Rect)
				}
			}
		}
	}
	for _, a := range w.agents {
		if z := w.zones[a.ZoneID]; z != nil && z.Rect.W > 0 {
			clampAgentToZone(a, z)
		}
	}
}

// remapAgent repositions an agent proportionally when its zone rect changes.
func remapAgent(a *Agent, old, cur Rect) {
	fx := (a.Pos.X - float32(old.X)) / float32(old.W)
	fy := (a.Pos.Y - float32(old.Y)) / float32(old.H)
	a.Pos.X = float32(cur.X) + fx*float32(cur.W)
	a.Pos.Y = float32(cur.Y) + fy*float32(cur.H)
	a.Target = a.Pos
	a.baseX, a.baseY = a.Pos.X, a.Pos.Y
}

// clampAgentToZone keeps an agent's sprite fully inside its zone interior.
func clampAgentToZone(a *Agent, z *Zone) {
	sw, sh := a.spriteSize()
	minX := float32(z.Rect.X + 1)
	maxX := float32(z.Rect.Right() - 1 - sw)
	minY := float32(z.Rect.Y + 1)
	maxY := float32(z.Rect.Bottom() - 1 - sh)
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	a.Pos.X = clampf(a.Pos.X, minX, maxX)
	a.Pos.Y = clampf(a.Pos.Y, minY, maxY)
}

// --- Snapshot views for rendering (value copies, read-locked) ---

// AgentView is an immutable snapshot of an agent for rendering.
type AgentView struct {
	ID        AgentID
	ParentID  AgentID
	Species   dex.Species
	Nick      string
	ZoneID    string
	Pos       Vec2
	Facing    int
	State     State
	Thought   Thought
	LastTool  string
	IsSub     bool
	WalkFrame int
	Clock     float32
	LastSeen  time.Time
}

// ZoneView is an immutable snapshot of a zone for rendering.
type ZoneView struct {
	ID    string
	Label string
	CWD   string
	Rect  Rect
	Kind  ZoneKind
	Deco  []Deco
}

// View is a full render snapshot taken under the read lock.
type View struct {
	Bounds    Rect
	Zones     []ZoneView  // only the zones on the current page
	ZoneTotal int         // total zones across all pages
	Agents    []AgentView // stable creation order
	Page      int
	PageCount int
	Now       time.Time
	Mode      Mode
	Space     SpaceView // populated only in Space Drift mode
}

// Snapshot returns a consistent copy of the world for rendering.
func (w *World) Snapshot(now time.Time) View {
	w.mu.RLock()
	defer w.mu.RUnlock()

	v := View{Bounds: w.bounds, Now: now, Page: w.page, ZoneTotal: len(w.zones), Mode: w.mode}
	if w.mode == ModeSpaceDrift {
		v.Space = w.space.snapshot()
	}
	n := len(w.zones)
	if n == 0 {
		v.PageCount = 1
	} else {
		v.PageCount = (n + zonesPerPage - 1) / zonesPerPage
	}

	for _, z := range w.zones {
		if z.Rect.W == 0 {
			continue // not on the current page
		}
		deco := make([]Deco, len(z.Deco))
		copy(deco, z.Deco)
		v.Zones = append(v.Zones, ZoneView{
			ID: z.ID, Label: z.Label, CWD: z.CWD,
			Rect: z.Rect, Kind: z.Kind, Deco: deco,
		})
	}
	sort.Slice(v.Zones, func(i, j int) bool { return v.Zones[i].ID < v.Zones[j].ID })

	for _, id := range w.order {
		a := w.agents[id]
		if a == nil {
			continue
		}
		v.Agents = append(v.Agents, AgentView{
			ID: a.ID, ParentID: a.ParentID, Species: a.Species,
			Nick: a.Nick, ZoneID: a.ZoneID, Pos: a.Pos, Facing: a.Facing,
			State: a.State, Thought: a.Thought, LastTool: a.LastTool,
			IsSub: a.IsSub(), WalkFrame: a.WalkFrame(), Clock: a.clock,
			LastSeen: a.LastSeen,
		})
	}
	return v
}

// AgentCount returns the number of live agents.
func (w *World) AgentCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.agents)
}

// ZoneCount returns the number of live zones.
func (w *World) ZoneCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.zones)
}
