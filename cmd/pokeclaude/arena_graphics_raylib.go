//go:build raylib

package main

import (
	"github.com/mpgxc/pokeclaude/internal/arena"
	"github.com/mpgxc/pokeclaude/internal/arena/rl"
)

// graphicsBuilt reports whether the raylib renderer is compiled in.
const graphicsBuilt = true

// runGraphics opens the raylib window and drives the match. The control server
// (if any) keeps running so an AI agent can steer a fighter live.
func runGraphics(m *arena.Arena, _ *arena.ControlServer) error {
	return rl.Run(m)
}
