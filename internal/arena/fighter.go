package arena

import "github.com/mpgxc/pokeclaude/internal/dex"

// State is a fighter's top-level status.
type State int

const (
	StateIdle    State = iota // standing / free to act
	StateWalk                 // moving
	StateAttack               // performing a move (see phase)
	StateBlock                // guarding
	StateDodge                // mid-dodge (i-frames early)
	StateHitstun              // stunned after being hit
	StateKO                   // knocked out this round
)

func (s State) String() string {
	switch s {
	case StateWalk:
		return "andando"
	case StateAttack:
		return "atacando"
	case StateBlock:
		return "bloqueando"
	case StateDodge:
		return "esquivando"
	case StateHitstun:
		return "atordoado"
	case StateKO:
		return "K.O."
	default:
		return "pronto"
	}
}

// attack phases within StateAttack
type phase int

const (
	phaseStartup phase = iota
	phaseActive
	phaseRecovery
)

func (p phase) String() string {
	switch p {
	case phaseActive:
		return "active"
	case phaseRecovery:
		return "recovery"
	default:
		return "startup"
	}
}

// Fighter is one combatant. Its behaviour is driven by the engine; a Controller
// only supplies Intents.
type Fighter struct {
	Side    int
	Species dex.Species
	Nick    string
	Stats   Stats
	Ctrl    Controller

	X      float64
	Facing int // +1 faces right, -1 faces left (toward opponent)

	HP        float64
	Mana      float64
	Meter     float64
	RoundsWon int

	State State
	move  Action
	ph    phase
	phT   float64 // time remaining in the current phase
	hit   bool    // whether the current active window already connected

	comboCount int
	hitstunT   float64
	dodgeT     float64 // remaining dodge duration
	dodgeCd    float64 // dodge cooldown
	dodgeDir   float64 // dash direction while dodging
	blockHeld  bool

	intent Intent
}

// newFighter builds a fighter for a species on a given side.
func newFighter(side int, sp dex.Species) *Fighter {
	f := &Fighter{
		Side:    side,
		Species: sp,
		Nick:    sp.Name,
		Stats:   StatsFor(sp),
	}
	f.resetForRound()
	return f
}

// resetForRound restores HP/mana/meter and neutral state at round start.
func (f *Fighter) resetForRound() {
	f.HP = f.Stats.MaxHP
	f.Mana = f.Stats.MaxMana
	f.Meter = 0
	f.State = StateIdle
	f.move = ActNone
	f.hit = false
	f.comboCount = 0
	f.hitstunT = 0
	f.dodgeT = 0
	f.dodgeCd = 0
	f.blockHeld = false
	f.intent = Intent{}
}

// alive reports whether the fighter still has HP this round.
func (f *Fighter) alive() bool { return f.HP > 0 }

// busy reports whether the fighter is locked into an animation and cannot start
// a new action.
func (f *Fighter) busy() bool {
	return f.State == StateAttack || f.State == StateHitstun ||
		f.State == StateDodge || f.State == StateKO
}

// invulnerable reports whether the fighter currently ignores hits (dodge
// i-frames cover the early part of the dodge).
func (f *Fighter) invulnerable() bool {
	return f.State == StateDodge && f.dodgeT > f.Stats.dodgeRecovery()
}

// meterFull reports whether the super meter is charged.
func (f *Fighter) meterFull() bool { return f.Meter >= 100 }

// addMeter clamps the super meter to [0,100].
func (f *Fighter) addMeter(v float64) {
	f.Meter += v
	if f.Meter > 100 {
		f.Meter = 100
	}
	if f.Meter < 0 {
		f.Meter = 0
	}
}
