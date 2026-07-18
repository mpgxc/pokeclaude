package arena

import (
	"testing"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

func speciesPair() (dex.Species, dex.Species) {
	all := dex.All()
	return all[0], all[1%len(all)]
}

func botMatch(seed int64) *Arena {
	a, b := speciesPair()
	return Build(Config{Seed: seed, RoundsToWin: 2},
		a, b, NewBot(seed+1, 0.5), NewBot(seed+2, 0.5))
}

func TestMatchCompletes(t *testing.T) {
	m := botMatch(7)
	// simulate up to ~5 minutes of fight; it must resolve well before that
	for i := 0; i < 60*300 && !m.Over(); i++ {
		m.Advance(1.0 / 60.0)
	}
	if !m.Over() {
		t.Fatal("match did not finish within the time budget")
	}
	if m.Winner() != 0 && m.Winner() != 1 {
		t.Errorf("no valid winner: %d", m.Winner())
	}
	sa, sb := m.Score()
	if sa < 2 && sb < 2 {
		t.Errorf("winner should have 2 rounds: %d–%d", sa, sb)
	}
}

func TestMatchDeterministic(t *testing.T) {
	run := func() (int, int, int) {
		m := botMatch(42)
		for i := 0; i < 60*300 && !m.Over(); i++ {
			m.Advance(1.0 / 60.0)
		}
		sa, sb := m.Score()
		return m.Winner(), sa, sb
	}
	w1, a1, b1 := run()
	w2, a2, b2 := run()
	if w1 != w2 || a1 != a2 || b1 != b2 {
		t.Errorf("non-deterministic: (%d %d-%d) vs (%d %d-%d)", w1, a1, b1, w2, a2, b2)
	}
}

// helper: drive a fight where A always performs `act` and B stands idle.
func scriptedAvsIdle(actA Action) *Arena {
	a, b := speciesPair()
	m := Build(Config{Seed: 1, RoundsToWin: 99}, a, b, NewRemote("a"), NewRemote("b"))
	// skip intro
	for m.phase == PhaseIntro {
		m.step(fixedDT)
	}
	m.A.Ctrl.(*RemoteController).Set(actA)
	return m
}

func TestLightAttackDealsDamage(t *testing.T) {
	m := scriptedAvsIdle(ActLight)
	// place them in range
	m.A.X, m.B.X = 40, 48
	startHP := m.B.HP
	for i := 0; i < 120; i++ { // ~1s
		m.step(fixedDT)
	}
	if m.B.HP >= startHP {
		t.Errorf("idle victim took no damage: %.1f -> %.1f", startHP, m.B.HP)
	}
}

func TestBlockReducesDamage(t *testing.T) {
	dmg := func(block bool) float64 {
		a, b := speciesPair()
		m := Build(Config{Seed: 1, RoundsToWin: 99}, a, b, NewRemote("a"), NewRemote("b"))
		for m.phase == PhaseIntro {
			m.step(fixedDT)
		}
		m.A.X, m.B.X = 40, 48
		m.A.Ctrl.(*RemoteController).Set(ActHeavy)
		if block {
			m.B.Ctrl.(*RemoteController).Set(ActBlock)
		}
		start := m.B.HP
		for i := 0; i < 120; i++ {
			m.step(fixedDT)
		}
		return start - m.B.HP
	}
	open := dmg(false)
	blocked := dmg(true)
	if open <= 0 {
		t.Fatal("expected damage when not blocking")
	}
	if blocked >= open {
		t.Errorf("block should reduce damage: open=%.1f blocked=%.1f", open, blocked)
	}
}

func TestDodgeIFramesIgnoreHit(t *testing.T) {
	a, b := speciesPair()
	// with dodge i-frames active, an otherwise-connecting hit deals no damage
	dodged := arenaHit(a, b, true)
	open := arenaHit(a, b, false)
	if open <= 0 {
		t.Fatal("control case should take damage")
	}
	if dodged != 0 {
		t.Errorf("dodge i-frames should ignore the hit, took %.1f", dodged)
	}
}

// arenaHit sets up A mid-active-heavy against B and returns the damage B takes,
// optionally with B in dodge i-frames.
func arenaHit(a, b dex.Species, dodging bool) float64 {
	m := Build(Config{Seed: 1, RoundsToWin: 99}, a, b, NewBot(1, 0), NewBot(2, 0))
	m.A.X, m.B.X = 40, 48
	m.A.State = StateAttack
	m.A.move = ActHeavy
	m.A.ph = phaseActive
	m.A.hit = false
	if dodging {
		m.B.State = StateDodge
		m.B.dodgeT = m.B.Stats.dodgeIFrames() + m.B.Stats.dodgeRecovery()
	}
	start := m.B.HP
	m.resolveHits()
	return start - m.B.HP
}

func TestSpecialCostsMana(t *testing.T) {
	m := scriptedAvsIdle(ActSpecial)
	m.A.X, m.B.X = 40, 50
	m.A.Mana = moves[ActSpecial].ManaCost + 1
	before := m.A.Mana
	// run past a decision poll (>0.25s) so the special actually fires
	for i := 0; i < 90; i++ {
		m.step(fixedDT)
	}
	if m.A.Mana >= before {
		t.Errorf("special should consume mana: %.1f -> %.1f", before, m.A.Mana)
	}
}

func TestKnockoutEndsRound(t *testing.T) {
	m := scriptedAvsIdle(ActHeavy)
	m.A.X, m.B.X = 40, 48
	m.B.HP = 5 // one hit should finish the round
	for i := 0; i < 240 && m.phase == PhaseFight; i++ {
		m.step(fixedDT)
		// keep A hammering
		if m.A.State == StateIdle {
			m.A.Ctrl.(*RemoteController).Set(ActHeavy)
		}
	}
	if m.scoreA != 1 {
		t.Errorf("A should have won the round, score=%d", m.scoreA)
	}
}

func TestStatsDeterministic(t *testing.T) {
	sp := dex.All()[0]
	s1 := StatsFor(sp)
	s2 := StatsFor(sp)
	if s1 != s2 {
		t.Error("StatsFor not deterministic")
	}
}

func TestParseAction(t *testing.T) {
	cases := map[string]Action{
		"leve": ActLight, "heavy": ActHeavy, "especial": ActSpecial,
		"bloquear": ActBlock, "esquiva": ActDodge, "super": ActSuper,
		"avancar": ActForward, "recuar": ActBack,
	}
	for s, want := range cases {
		if got, ok := ParseAction(s); !ok || got != want {
			t.Errorf("ParseAction(%q) = %v,%v want %v", s, got, ok, want)
		}
	}
	if _, ok := ParseAction("banana"); ok {
		t.Error("unknown action should not parse")
	}
}
