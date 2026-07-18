package arena

import (
	"fmt"
	"math/rand"

	"github.com/mpgxc/pokeclaude/internal/dex"
)

// Engine timing / geometry constants.
const (
	fixedDT          = 1.0 / 120.0
	decisionInterval = 0.25 // controllers are polled 4x/second
	introTime        = 1.6
	roundOverTime    = 2.4
	roundTime        = 60.0
	fighterHalf      = 4.0
	minSeparation    = 8.0
	wallMargin       = 4.0
	dodgeSpeed       = 42.0
	shakeDecay       = 14.0
	maxEvents        = 10
)

// MatchPhase is the top-level flow state of a match.
type MatchPhase int

const (
	PhaseIntro MatchPhase = iota
	PhaseFight
	PhaseRoundOver
	PhaseMatchOver
)

// Config parameters a match.
type Config struct {
	Width        float64 // arena width in logical units
	RoundsToWin  int     // wins needed to take the match (best of 2N-1)
	Seed         int64
	RoundSeconds float64 // per-round time limit; 0 = default, negative = no limit
}

func (c Config) withDefaults() Config {
	if c.Width <= 0 {
		c.Width = 100
	}
	if c.RoundsToWin <= 0 {
		c.RoundsToWin = 2
	}
	if c.RoundSeconds == 0 {
		c.RoundSeconds = roundTime
	}
	return c
}

// spark is a transient hit effect for the renderer.
type spark struct {
	x, y, age, ttl float64
	big            bool
}

// Arena is a full match: two fighters, round/score flow and effects.
type Arena struct {
	cfg   Config
	Width float64
	A, B  *Fighter
	rng   *rand.Rand

	round   int
	scoreA  int
	scoreB  int
	phase   MatchPhase
	phaseT  float64
	roundT  float64
	decAcc  float64
	endless bool // no round time limit (waits for input; only KO ends a round)

	events   []string // rolling, for the HUD
	allLog   []string // full history, for headless output
	sparks   []spark
	shake    float64
	announce string
	winner   int // -1 none, else side
	tookDmg  [2]bool
}

// Build assembles a match for two species with the given controllers.
func Build(cfg Config, spA, spB dex.Species, ctrlA, ctrlB Controller) *Arena {
	cfg = cfg.withDefaults()
	a := newFighter(0, spA)
	b := newFighter(1, spB)
	a.Ctrl, b.Ctrl = ctrlA, ctrlB
	m := &Arena{
		cfg:     cfg,
		Width:   cfg.Width,
		A:       a,
		B:       b,
		rng:     rand.New(rand.NewSource(cfg.Seed)),
		winner:  -1,
		endless: cfg.RoundSeconds < 0,
	}
	m.startRound(1)
	return m
}

// startRound resets both fighters and enters the intro for round n.
func (m *Arena) startRound(n int) {
	m.round = n
	m.A.resetForRound()
	m.B.resetForRound()
	m.A.X = m.Width * 0.3
	m.B.X = m.Width * 0.7
	m.tookDmg = [2]bool{}
	m.phase = PhaseIntro
	m.phaseT = introTime
	if m.endless {
		m.roundT = 99 // display only; never decremented
	} else {
		m.roundT = m.cfg.RoundSeconds
	}
	m.announce = fmt.Sprintf("ROUND %d — LUTAR!", n)
}

// Advance steps the match forward by dt seconds using a fixed internal step.
func (m *Arena) Advance(dt float64) {
	for dt > 0 {
		step := fixedDT
		if dt < step {
			step = dt
		}
		m.step(step)
		dt -= step
	}
}

func (m *Arena) step(dt float64) {
	// decay effects
	if m.shake > 0 {
		m.shake -= shakeDecay * dt
		if m.shake < 0 {
			m.shake = 0
		}
	}
	m.ageSparks(dt)

	switch m.phase {
	case PhaseIntro:
		m.phaseT -= dt
		if m.phaseT <= 0 {
			m.phase = PhaseFight
			m.announce = ""
		}
	case PhaseFight:
		m.fightStep(dt)
	case PhaseRoundOver:
		m.phaseT -= dt
		if m.phaseT <= 0 {
			m.afterRound()
		}
	case PhaseMatchOver:
		// idle
	}
}

func (m *Arena) fightStep(dt float64) {
	// round timer (skipped in endless mode: the round only ends on a K.O.)
	if !m.endless {
		m.roundT -= dt
		if m.roundT <= 0 {
			m.endRoundByTimeout()
			return
		}
	}

	// poll controllers at the decision cadence
	m.decAcc += dt
	if m.decAcc >= decisionInterval {
		m.decAcc -= decisionInterval
		m.A.intent = m.A.Ctrl.Decide(m.viewOf(m.A), m.viewOf(m.B))
		m.B.intent = m.B.Ctrl.Decide(m.viewOf(m.B), m.viewOf(m.A))
	}

	m.updateFighter(m.A, m.B, dt)
	m.updateFighter(m.B, m.A, dt)
	m.resolveHits()

	if !m.A.alive() {
		m.endRound(1)
	} else if !m.B.alive() {
		m.endRound(0)
	}
}

// updateFighter advances one fighter's state machine.
func (m *Arena) updateFighter(f, opp *Fighter, dt float64) {
	// mana regen
	f.Mana += f.Stats.ManaRegen * dt
	if f.Mana > f.Stats.MaxMana {
		f.Mana = f.Stats.MaxMana
	}
	if f.dodgeCd > 0 {
		f.dodgeCd -= dt
	}
	// always face the opponent
	if opp.X >= f.X {
		f.Facing = 1
	} else {
		f.Facing = -1
	}

	switch f.State {
	case StateKO:
		return
	case StateHitstun:
		f.hitstunT -= dt
		if f.hitstunT <= 0 {
			f.State = StateIdle
		}
		return
	case StateDodge:
		f.X += f.dodgeDir * dodgeSpeed * dt
		m.clampX(f, opp)
		f.dodgeT -= dt
		if f.dodgeT <= 0 {
			f.State = StateIdle
		}
		return
	case StateAttack:
		m.advanceAttack(f, dt)
		return
	}

	// free to act (Idle / Walk / Block): obey the current intent
	m.applyFreeIntent(f, opp, dt)
}

func (m *Arena) applyFreeIntent(f, opp *Fighter, dt float64) {
	switch f.intent.Action {
	case ActBlock:
		f.State = StateBlock
		f.blockHeld = true
	case ActForward:
		f.State = StateWalk
		f.blockHeld = false
		f.X += float64(f.Facing) * f.Stats.moveSpeed() * dt
		m.clampX(f, opp)
	case ActBack:
		f.State = StateWalk
		f.blockHeld = false
		f.X -= float64(f.Facing) * f.Stats.moveSpeed() * dt
		m.clampX(f, opp)
	case ActDodge:
		if f.dodgeCd <= 0 {
			f.State = StateDodge
			f.dodgeT = f.Stats.dodgeIFrames() + f.Stats.dodgeRecovery()
			f.dodgeCd = f.dodgeT + 0.15
			f.dodgeDir = float64(f.Facing) // dash forward through/past
			f.blockHeld = false
		}
	case ActLight, ActHeavy, ActSpecial, ActSuper:
		m.startAttack(f)
	default:
		f.State = StateIdle
		f.blockHeld = false
	}
}

// startAttack begins an attack if the fighter can pay for it.
func (m *Arena) startAttack(f *Fighter) {
	a := f.intent.Action
	spec := moves[a]
	if a == ActSpecial && f.Mana < spec.ManaCost {
		f.State = StateIdle
		return
	}
	if a == ActSuper && !f.meterFull() {
		f.State = StateIdle
		return
	}
	if a == ActSpecial {
		f.Mana -= spec.ManaCost
	}
	if a == ActSuper {
		f.Meter = 0
	}
	f.State = StateAttack
	f.move = a
	f.ph = phaseStartup
	f.phT = spec.Startup
	f.hit = false
	f.blockHeld = false
}

func (m *Arena) advanceAttack(f *Fighter, dt float64) {
	f.phT -= dt
	if f.phT > 0 {
		return
	}
	spec := moves[f.move]
	switch f.ph {
	case phaseStartup:
		f.ph = phaseActive
		f.phT = spec.Active
	case phaseActive:
		f.ph = phaseRecovery
		f.phT = spec.Recovery
	case phaseRecovery:
		f.State = StateIdle
		f.move = ActNone
	}
}

// resolveHits checks each fighter's live hitbox against the opponent.
func (m *Arena) resolveHits() {
	m.tryHit(m.A, m.B)
	m.tryHit(m.B, m.A)
}

func (m *Arena) tryHit(att, vic *Fighter) {
	if att.State != StateAttack || att.ph != phaseActive || att.hit {
		return
	}
	spec := moves[att.move]
	dist := att.X - vic.X
	if dist < 0 {
		dist = -dist
	}
	if dist > spec.Reach {
		return
	}
	att.hit = true

	if vic.invulnerable() {
		m.logf("%s esquiva do %s de %s!", vic.Nick, spec.Name, att.Nick)
		return
	}

	// combo bookkeeping (a hit while the victim is already stunned extends it)
	if vic.State == StateHitstun {
		att.comboCount++
	} else {
		att.comboCount = 1
	}

	raw := spec.Damage * att.Stats.attackMult() * att.Stats.comboScale(att.comboCount)
	blocked := vic.State == StateBlock
	if blocked {
		raw *= spec.BlockChip
	}
	dmg := vic.Stats.damageTaken(raw)
	vic.HP -= dmg
	if vic.HP < 0 {
		vic.HP = 0
	}
	m.tookDmg[vic.Side] = true

	// knockback
	push := spec.Knockback
	if blocked {
		push *= 0.4
	}
	if vic.X < att.X {
		vic.X -= push
	} else {
		vic.X += push
	}
	m.clampX(vic, att)

	// stun + meter + effects
	if !blocked {
		vic.State = StateHitstun
		vic.hitstunT = spec.Hitstun
	}
	att.addMeter(spec.Meter)
	vic.addMeter(spec.Meter * 0.5)
	m.spawnSpark(vic.X, blocked || spec.Damage >= 14)
	m.shake += spec.Knockback * 0.6
	if m.shake > 10 {
		m.shake = 10
	}

	combo := ""
	if att.comboCount > 1 {
		combo = fmt.Sprintf(" (combo x%d)", att.comboCount)
	}
	switch {
	case blocked:
		m.logf("%s BLOQUEIA o %s (-%.0f)", vic.Nick, spec.Name, dmg)
	case att.move == ActSuper:
		m.logf("💥 %s desfere a FINALIZAÇÃO em %s (-%.0f)!", att.Nick, vic.Nick, dmg)
	default:
		m.logf("%s acerta %s com %s -%.0f%s", att.Nick, vic.Nick, spec.Name, dmg, combo)
	}
}

// clampX keeps a fighter within the walls and out of the opponent's body.
func (m *Arena) clampX(f, other *Fighter) {
	lo := wallMargin
	hi := m.Width - wallMargin
	if f.X < lo {
		f.X = lo
	}
	if f.X > hi {
		f.X = hi
	}
	// keep a minimum separation
	if d := f.X - other.X; d >= 0 && d < minSeparation {
		f.X = other.X + minSeparation
	} else if d < 0 && -d < minSeparation {
		f.X = other.X - minSeparation
	}
	if f.X < lo {
		f.X = lo
	}
	if f.X > hi {
		f.X = hi
	}
}

// endRound resolves a knockout: winner is the given side.
func (m *Arena) endRound(winner int) {
	loser := m.A
	win := m.B
	if winner == 0 {
		loser, win = m.B, m.A
	}
	loser.State = StateKO
	if winner == 0 {
		m.scoreA++
	} else {
		m.scoreB++
	}
	flawless := !m.tookDmg[winner]
	if flawless {
		m.announce = fmt.Sprintf("%s — VITÓRIA IMPECÁVEL!", win.Nick)
	} else {
		m.announce = fmt.Sprintf("K.O.! %s vence o round", win.Nick)
	}
	m.logf("%s", m.announce)
	m.enterRoundOver(winner)
}

// endRoundByTimeout awards the round to whoever has more HP.
func (m *Arena) endRoundByTimeout() {
	winner := 0
	if m.B.HP > m.A.HP {
		winner = 1
	} else if m.A.HP == m.B.HP {
		// sudden draw: award to higher meter, else side A
		if m.B.Meter > m.A.Meter {
			winner = 1
		}
	}
	if winner == 0 {
		m.scoreA++
	} else {
		m.scoreB++
	}
	name := m.A.Nick
	if winner == 1 {
		name = m.B.Nick
	}
	m.announce = fmt.Sprintf("TEMPO! %s vence por vida", name)
	m.logf("%s", m.announce)
	m.enterRoundOver(winner)
}

func (m *Arena) enterRoundOver(winner int) {
	if winner == 0 {
		m.A.RoundsWon = m.scoreA
	} else {
		m.B.RoundsWon = m.scoreB
	}
	m.phase = PhaseRoundOver
	m.phaseT = roundOverTime
}

// afterRound advances to the next round or ends the match.
func (m *Arena) afterRound() {
	if m.scoreA >= m.cfg.RoundsToWin {
		m.finishMatch(0)
		return
	}
	if m.scoreB >= m.cfg.RoundsToWin {
		m.finishMatch(1)
		return
	}
	m.startRound(m.round + 1)
}

func (m *Arena) finishMatch(winner int) {
	m.phase = PhaseMatchOver
	m.winner = winner
	name := m.A.Nick
	if winner == 1 {
		name = m.B.Nick
	}
	m.announce = fmt.Sprintf("%s VENCE A PARTIDA! (%d–%d)", name, m.scoreA, m.scoreB)
	m.logf("%s", m.announce)
}

// --- effects & log ---

func (m *Arena) spawnSpark(x float64, big bool) {
	y := 6 + m.rng.Float64()*3
	ttl := 0.25
	if big {
		ttl = 0.4
	}
	m.sparks = append(m.sparks, spark{x: x, y: y, ttl: ttl, big: big})
}

func (m *Arena) ageSparks(dt float64) {
	kept := m.sparks[:0]
	for _, s := range m.sparks {
		s.age += dt
		if s.age < s.ttl {
			kept = append(kept, s)
		}
	}
	m.sparks = kept
}

func (m *Arena) logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	m.allLog = append(m.allLog, line)
	m.events = append(m.events, line)
	if len(m.events) > maxEvents {
		m.events = m.events[len(m.events)-maxEvents:]
	}
}

// Log returns the full, untrimmed event history.
func (m *Arena) Log() []string { return m.allLog }

// viewOf builds a controller-facing snapshot.
func (m *Arena) viewOf(f *Fighter) FighterView {
	return FighterView{
		Side: f.Side, Species: f.Species, Nick: f.Nick,
		X: f.X, Facing: f.Facing,
		HP: f.HP, MaxHP: f.Stats.MaxHP, Mana: f.Mana, Meter: f.Meter,
		RoundsWon: f.RoundsWon, State: f.State, Move: f.move,
		ComboCount: f.comboCount, Busy: f.busy(), MeterFull: f.meterFull(),
	}
}

// --- public accessors ---

// Over reports whether the match has finished.
func (m *Arena) Over() bool { return m.phase == PhaseMatchOver }

// Winner returns the winning side (0/1), or -1 if undecided.
func (m *Arena) Winner() int { return m.winner }

// Score returns each side's rounds won.
func (m *Arena) Score() (int, int) { return m.scoreA, m.scoreB }

// Events returns the rolling event log.
func (m *Arena) Events() []string { return m.events }
