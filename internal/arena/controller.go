package arena

import (
	"math/rand"
	"sync"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

// FighterView is a read-only snapshot handed to a Controller each decision tick.
type FighterView struct {
	Side       int
	Species    dex.Species
	Nick       string
	X          float64
	Facing     int
	HP, MaxHP  float64
	Mana       float64
	Meter      float64
	RoundsWon  int
	State      State
	Move       Action
	ComboCount int
	Busy       bool
	MeterFull  bool
}

// Controller decides a fighter's next Intent from its own and the opponent's
// view. Decide is polled at a fixed cadence (see decisionInterval), not every
// physics frame, so an LLM-backed controller has time to think.
type Controller interface {
	Decide(me, opp FighterView) Intent
	Name() string
}

// BotController is a deterministic heuristic opponent. Given the same seed and
// inputs it always plays the same, which keeps simulations reproducible.
type BotController struct {
	rng        *rand.Rand
	aggression float64 // 0..1
}

// NewBot builds a heuristic controller seeded deterministically.
func NewBot(seed int64, aggression float64) *BotController {
	return &BotController{rng: rand.New(rand.NewSource(seed)), aggression: aggression}
}

func (b *BotController) Name() string { return "bot" }

// Decide implements a simple neutral game: close distance, poke in range,
// react to incoming attacks with block/dodge, spend meter and mana when it can.
func (b *BotController) Decide(me, opp FighterView) Intent {
	if me.Busy {
		return Intent{ActNone}
	}
	dist := me.X - opp.X
	if dist < 0 {
		dist = -dist
	}

	// react to an incoming attack when close
	if opp.State == StateAttack && dist < 14 {
		r := b.rng.Float64()
		switch {
		case r < 0.35:
			return Intent{ActDodge}
		case r < 0.8:
			return Intent{ActBlock}
		}
	}

	// finisher when charged and in range
	if me.MeterFull && dist < moves[ActSuper].Reach {
		return Intent{ActSuper}
	}

	// approach if out of poke range
	if dist > moves[ActHeavy].Reach {
		// occasionally dash-dodge forward to close faster
		if b.rng.Float64() < 0.1 {
			return Intent{ActDodge}
		}
		return Intent{ActForward}
	}

	// in range: pick an offensive option
	r := b.rng.Float64() * (0.5 + b.aggression)
	switch {
	case me.Mana >= moves[ActSpecial].ManaCost && r > 0.85:
		return Intent{ActSpecial}
	case r > 0.55:
		return Intent{ActHeavy}
	case r > 0.15:
		return Intent{ActLight}
	default:
		return Intent{ActBlock}
	}
}

// RemoteController is fed Intents from the outside (a live AI agent over the
// control socket, or a CLI). One-shot actions (attacks, dodge, super) fire once
// and reset; held actions (walk, block, idle) persist until changed.
type RemoteController struct {
	mu   sync.Mutex
	cur  Action
	name string
}

// NewRemote builds an externally-driven controller.
func NewRemote(name string) *RemoteController {
	if name == "" {
		name = "remote"
	}
	return &RemoteController{name: name}
}

func (r *RemoteController) Name() string { return r.name }

// Set updates the pending action from the outside world.
func (r *RemoteController) Set(a Action) {
	r.mu.Lock()
	r.cur = a
	r.mu.Unlock()
}

// Decide returns the pending action. One-shot actions are consumed (reset to
// ActNone) so a single command produces a single move; held actions persist.
func (r *RemoteController) Decide(me, opp FighterView) Intent {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.cur
	if oneShot(a) {
		r.cur = ActNone
	}
	return Intent{a}
}

func oneShot(a Action) bool {
	switch a {
	case ActLight, ActHeavy, ActSpecial, ActSuper, ActDodge:
		return true
	}
	return false
}

// ParseAction maps a command string (from the control CLI/socket) to an Action.
func ParseAction(s string) (Action, bool) {
	switch s {
	case "none", "idle", "parado", "":
		return ActNone, true
	case "forward", "avancar", "avançar", "aproximar", "frente":
		return ActForward, true
	case "back", "recuar", "tras", "trás":
		return ActBack, true
	case "light", "leve", "jab", "soco":
		return ActLight, true
	case "heavy", "pesado", "chute":
		return ActHeavy, true
	case "special", "especial":
		return ActSpecial, true
	case "block", "bloquear", "defesa", "guarda":
		return ActBlock, true
	case "dodge", "esquiva", "esquivar":
		return ActDodge, true
	case "super", "finalizacao", "finalização", "fatality":
		return ActSuper, true
	default:
		return ActNone, false
	}
}
