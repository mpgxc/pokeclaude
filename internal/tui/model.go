// Package tui hosts the bubbletea game loop that drives the world and renders
// it at 20fps.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mpgxc/pokeclaude/internal/protocol"
	"github.com/mpgxc/pokeclaude/internal/render"
	"github.com/mpgxc/pokeclaude/internal/world"
)

const tickInterval = 50 * time.Millisecond // 20fps

// Model is the bubbletea model: a world plus the event channel feeding it.
type Model struct {
	world  *world.World
	events <-chan protocol.Envelope
	w, h   int
	paused bool
	last   time.Time
}

// Option customizes a Model at construction.
type Option func(*Model)

// WithMode sets the initial movement mode (e.g. Space Drift).
func WithMode(mode world.Mode) Option {
	return func(m *Model) { m.world.SetMode(mode) }
}

// New creates a Model reading events from ch.
func New(ch <-chan protocol.Envelope, opts ...Option) Model {
	m := Model{
		world:  world.New(world.Rect{X: 1, Y: 3, W: 78, H: 14}),
		events: ch,
		last:   time.Now(),
	}
	for _, o := range opts {
		o(&m)
	}
	return m
}

// Messages driving the loop.
type (
	tickMsg  time.Time
	eventMsg protocol.Envelope
)

// tick schedules the next animation frame.
func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// waitEvent blocks (in its own goroutine, managed by bubbletea) for the next
// envelope from the ingest channel.
func waitEvent(ch <-chan protocol.Envelope) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(e)
	}
}

// Init starts the tick loop and the event listener.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), waitEvent(m.events))
}

// View renders the current frame.
func (m Model) View() string {
	if m.w == 0 || m.h == 0 {
		return "iniciando PokéClaude…"
	}
	v := m.world.Snapshot(time.Now())
	return render.RenderFrame(v, m.w, m.h)
}
