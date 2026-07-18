//go:build !raylib

package main

import (
	"errors"

	"github.com/mpgxc/pokeclaude/internal/arena"
)

// graphicsBuilt reports whether the raylib renderer is compiled in.
const graphicsBuilt = false

// runGraphics is a stub when the binary is built without raylib.
func runGraphics(_ *arena.Arena, _ *arena.ControlServer) error {
	return errors.New("binário compilado sem raylib — recompile com `-tags raylib` ou use --headless")
}
