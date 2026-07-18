package arena

import "github.com/mpgxc/pokeclaude/internal/dex"

// ViewFighter is an immutable render snapshot of a fighter.
type ViewFighter struct {
	Side               int
	Species            dex.Species
	Nick               string
	X                  float64
	Facing             int
	HP, MaxHP          float64
	Mana, MaxMana      float64
	Meter              float64
	RoundsWon          int
	State              State
	Move               Action
	Phase              string
	ComboCount         int
	Blocking, Dodging  bool
	Attacking, Stunned bool
	KO                 bool
}

// Spark is a transient hit effect for the renderer.
type Spark struct {
	X, Y float64
	Age  float64
	TTL  float64
	Big  bool
}

// MatchView is a full render snapshot of the match.
type MatchView struct {
	Width       float64
	Round       int
	RoundsToWin int
	ScoreA      int
	ScoreB      int
	A, B        ViewFighter
	Phase       MatchPhase
	Announce    string
	TimeLeft    float64
	Shake       float64
	Sparks      []Spark
	Events      []string
	Over        bool
	Winner      int
}

// View returns a consistent render snapshot of the match.
func (m *Arena) View() MatchView {
	sparks := make([]Spark, len(m.sparks))
	for i, s := range m.sparks {
		sparks[i] = Spark{X: s.x, Y: s.y, Age: s.age, TTL: s.ttl, Big: s.big}
	}
	ev := make([]string, len(m.events))
	copy(ev, m.events)
	return MatchView{
		Width:       m.Width,
		Round:       m.round,
		RoundsToWin: m.cfg.RoundsToWin,
		ScoreA:      m.scoreA,
		ScoreB:      m.scoreB,
		A:           viewFighter(m.A),
		B:           viewFighter(m.B),
		Phase:       m.phase,
		Announce:    m.announce,
		TimeLeft:    m.roundT,
		Shake:       m.shake,
		Sparks:      sparks,
		Events:      ev,
		Over:        m.phase == PhaseMatchOver,
		Winner:      m.winner,
	}
}

func viewFighter(f *Fighter) ViewFighter {
	return ViewFighter{
		Side: f.Side, Species: f.Species, Nick: f.Nick,
		X: f.X, Facing: f.Facing,
		HP: f.HP, MaxHP: f.Stats.MaxHP,
		Mana: f.Mana, MaxMana: f.Stats.MaxMana, Meter: f.Meter,
		RoundsWon: f.RoundsWon,
		State:     f.State, Move: f.move, Phase: f.ph.String(),
		ComboCount: f.comboCount,
		Blocking:   f.State == StateBlock,
		Dodging:    f.State == StateDodge,
		Attacking:  f.State == StateAttack,
		Stunned:    f.State == StateHitstun,
		KO:         f.State == StateKO,
	}
}
