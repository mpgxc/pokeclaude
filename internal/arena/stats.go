package arena

import (
	"hash/fnv"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

// Stats is a fighter's build. Values are on a ~1..10 scale and are turned into
// concrete engine numbers by the derived accessors below.
type Stats struct {
	MaxHP     float64
	MaxMana   float64
	Attack    float64 // 1..10 damage multiplier basis
	Defense   float64 // 1..10 incoming-damage mitigation basis
	Speed     float64 // 1..10 move speed + dodge quality + startup
	Combo     float64 // 1..10 combo damage-scaling resistance
	ManaRegen float64 // mana per second
}

// StatsFor derives a deterministic build from a species, so the same Pokémon
// always fights the same way. Points are spread from the species name hash but
// kept within a balanced band so no build is hopeless.
func StatsFor(sp dex.Species) Stats {
	h := fnv.New64a()
	_, _ = h.Write([]byte(sp.Name))
	seed := h.Sum64()

	// four stats in [3,9], derived from disjoint bit slices of the hash
	atk := 3 + float64((seed>>0)%7)
	def := 3 + float64((seed>>8)%7)
	spd := 3 + float64((seed>>16)%7)
	cmb := 3 + float64((seed>>24)%7)

	return Stats{
		MaxHP:     100,
		MaxMana:   100,
		Attack:    atk,
		Defense:   def,
		Speed:     spd,
		Combo:     cmb,
		ManaRegen: 8 + spd, // faster fighters recover mana quicker
	}
}

// moveSpeed converts the Speed stat into arena units per second.
func (s Stats) moveSpeed() float64 { return 14 + s.Speed*2.2 }

// dodgeIFrames returns how long an esquiva grants invulnerability.
func (s Stats) dodgeIFrames() float64 { return 0.16 + s.Speed*0.012 }

// dodgeRecovery is the cooldown/recovery of a dodge.
func (s Stats) dodgeRecovery() float64 { return 0.34 - s.Speed*0.012 }

// attackMult scales base move damage by the Attack stat (1.0 at Attack 5).
func (s Stats) attackMult() float64 { return 0.6 + s.Attack*0.08 }

// damageTaken applies Defense mitigation to an incoming raw damage value.
func (s Stats) damageTaken(raw float64) float64 {
	mit := 1.0 - s.Defense*0.05 // Defense 9 => 0.55x damage taken
	if mit < 0.35 {
		mit = 0.35
	}
	return raw * mit
}

// comboScale returns the damage multiplier for the nth hit of a combo. Higher
// Combo stat keeps later hits hurting.
func (s Stats) comboScale(n int) float64 {
	if n <= 1 {
		return 1.0
	}
	drop := 0.18 - s.Combo*0.012 // better Combo => gentler falloff
	if drop < 0.05 {
		drop = 0.05
	}
	scale := 1.0 - float64(n-1)*drop
	floor := 0.3 + s.Combo*0.03
	if scale < floor {
		scale = floor
	}
	return scale
}
