package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mpgxc/pokeclaude/internal/protocol"
	"github.com/mpgxc/pokeclaude/internal/render"
)

// toEnvelope converts an eventMsg back to its underlying envelope.
func toEnvelope(m eventMsg) protocol.Envelope { return protocol.Envelope(m) }

// Update handles ticks, incoming events, resizes and key presses.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		now := time.Time(msg)
		if !m.paused {
			dt := float32(now.Sub(m.last).Seconds())
			if dt <= 0 || dt > 0.5 {
				dt = float32(tickInterval.Seconds())
			}
			m.world.Tick(dt, now)
		}
		m.last = now
		return m, tick()

	case eventMsg:
		m.world.Apply(toEnvelope(msg))
		return m, waitEvent(m.events)

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		lay := render.ComputeLayout(msg.Width, msg.Height)
		m.world.Resize(lay.Map)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			m.paused = !m.paused
		case "tab":
			m.world.NextPage()
		}
	}
	return m, nil
}
