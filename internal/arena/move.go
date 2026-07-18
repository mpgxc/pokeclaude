// Package arena is the pure, deterministic combat engine behind PokéClaude's
// "Arena" game mode — a Mortal-Kombat-style 1v1 fight between two pets. It has
// no rendering or I/O dependencies so the whole fight can be simulated and unit
// tested headless; the raylib renderer and the control socket sit on top.
package arena

// Action is a single control input a fighter can express on a decision tick.
type Action int

const (
	ActNone    Action = iota // do nothing / stand
	ActForward               // walk toward the opponent
	ActBack                  // walk away from the opponent
	ActLight                 // fast jab (comboable)
	ActHeavy                 // slow heavy hit (knockback)
	ActSpecial               // costs mana, high damage
	ActBlock                 // guard (reduces damage, chip only)
	ActDodge                 // short i-frame dash (esquiva)
	ActSuper                 // finisher, requires a full meter
)

// String returns a short label for logs/HUD.
func (a Action) String() string {
	switch a {
	case ActForward:
		return "avançar"
	case ActBack:
		return "recuar"
	case ActLight:
		return "leve"
	case ActHeavy:
		return "pesado"
	case ActSpecial:
		return "especial"
	case ActBlock:
		return "bloquear"
	case ActDodge:
		return "esquiva"
	case ActSuper:
		return "SUPER"
	default:
		return "parado"
	}
}

// Intent is what a Controller returns each decision tick.
type Intent struct {
	Action Action
}

// MoveSpec is the frame data and effect of an attacking action. Times are in
// seconds; a fixed-step engine turns them into frames.
type MoveSpec struct {
	Action    Action
	Name      string
	Startup   float64 // windup before the hitbox is live
	Active    float64 // hitbox live
	Recovery  float64 // vulnerable cooldown after
	Damage    float64 // base, scaled by the attacker's Attack stat
	Reach     float64 // horizontal range in front
	Knockback float64 // push applied to the victim
	Hitstun   float64 // how long the victim is stunned (enables combos)
	ManaCost  float64
	Meter     float64 // super meter gained on connect
	BlockChip float64 // fraction of damage dealt through a block
}

// moves is the attack table. Values are tuned for readable, comboable fights.
var moves = map[Action]MoveSpec{
	ActLight: {
		Action: ActLight, Name: "soco leve",
		Startup: 0.06, Active: 0.05, Recovery: 0.12,
		Damage: 6, Reach: 9, Knockback: 1.0, Hitstun: 0.28,
		ManaCost: 0, Meter: 6, BlockChip: 0.12,
	},
	ActHeavy: {
		Action: ActHeavy, Name: "chute pesado",
		Startup: 0.16, Active: 0.07, Recovery: 0.30,
		Damage: 14, Reach: 11, Knockback: 6.0, Hitstun: 0.45,
		ManaCost: 0, Meter: 12, BlockChip: 0.2,
	},
	ActSpecial: {
		Action: ActSpecial, Name: "golpe especial",
		Startup: 0.12, Active: 0.10, Recovery: 0.34,
		Damage: 22, Reach: 16, Knockback: 8.0, Hitstun: 0.5,
		ManaCost: 35, Meter: 16, BlockChip: 0.28,
	},
	ActSuper: {
		Action: ActSuper, Name: "FINALIZAÇÃO",
		Startup: 0.18, Active: 0.12, Recovery: 0.5,
		Damage: 40, Reach: 18, Knockback: 12.0, Hitstun: 0.7,
		ManaCost: 0, Meter: 0, BlockChip: 0.4,
	},
}

// isAttack reports whether an action launches an attack move.
func isAttack(a Action) bool {
	switch a {
	case ActLight, ActHeavy, ActSpecial, ActSuper:
		return true
	}
	return false
}
