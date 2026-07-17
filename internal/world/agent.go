package world

import (
	"time"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

// AgentID identifies an agent. For a root session it equals the session id; a
// subagent uses parentID + "#" + n.
type AgentID string

// State is the behavioural state of an agent, driving both animation and the
// thought bubble.
type State int

const (
	StateIdle     State = iota // wandering
	StateThinking              // stationary, pulsing "..."
	StateWorking               // vertical bounce, bubble shows the tool
	StateError                 // horizontal shake, bubble shows 💥
	StateDone                  // happy, fades out after a delay
)

// String returns a short label used in the HUD.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateThinking:
		return "Thinking"
	case StateWorking:
		return "Working"
	case StateError:
		return "Error"
	case StateDone:
		return "Done"
	default:
		return "?"
	}
}

// Thought is the content of an agent's speech bubble.
type Thought struct {
	Text    string
	Icon    rune
	Expires time.Time // zero => sticky until the next event replaces it
}

// Active reports whether the thought should still be shown at time now.
func (t Thought) Active(now time.Time) bool {
	if t.Text == "" {
		return false
	}
	return t.Expires.IsZero() || now.Before(t.Expires)
}

// Agent is one Pokémon on the map, representing a Claude Code session or
// subagent.
type Agent struct {
	ID       AgentID
	ParentID AgentID // "" for a root session; set => subagent (smaller sprite)
	Species  dex.Species
	Nick     string
	ZoneID   string

	Pos    Vec2
	Target Vec2
	Facing int // -1 left, +1 right
	State  State

	Thought  Thought
	LastTool string

	LastSeen  time.Time
	SpawnedAt time.Time

	// animation / behaviour bookkeeping (seconds)
	clock      float32   // ever-increasing animation clock
	stateSince float32   // clock value when the current state began
	dwell      float32   // remaining idle dwell time before picking a new target
	baseY      float32   // resting Y for the working bounce
	baseX      float32   // resting X for the error shake
	fadeAt     time.Time // when a Done/despawning agent should disappear
	subCount   int       // number of subagents spawned (for id generation)
	removing   bool      // marked for removal after fade
}

// IsSub reports whether this agent is a subagent.
func (a *Agent) IsSub() bool { return a.ParentID != "" }

// setState transitions to a new state, resetting per-state bookkeeping.
func (a *Agent) setState(s State) {
	if a.State != s {
		a.State = s
		a.stateSince = a.clock
	}
	a.baseX = a.Pos.X
	a.baseY = a.Pos.Y
}

// stateAge returns how long (seconds) the agent has been in its current state.
func (a *Agent) stateAge() float32 { return a.clock - a.stateSince }
