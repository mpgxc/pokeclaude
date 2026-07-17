package main

import (
	"fmt"
	"strings"

	"github.com/mpgxc/pokeclaude/internal/tui"
	"github.com/mpgxc/pokeclaude/internal/world"
)

// parseModeOptions turns a --mode flag value into TUI options.
func parseModeOptions(mode string) ([]tui.Option, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "zone", "zonas", "zona":
		return nil, nil
	case "space", "spacedrift", "space-drift", "drift", "espaco", "espaço":
		return []tui.Option{tui.WithMode(world.ModeSpaceDrift)}, nil
	default:
		return nil, fmt.Errorf("modo desconhecido %q (use \"zona\" ou \"space\")", mode)
	}
}
