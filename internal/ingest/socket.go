// Package ingest hosts the unix-socket server that receives hook envelopes from
// the shim, and provides the socket path resolution shared with the shim.
package ingest

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// SocketPath returns the unix socket path, preferring $XDG_RUNTIME_DIR and
// falling back to a per-uid file in $TMPDIR (or /tmp).
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "pokeclaude.sock")
	}
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	return filepath.Join(tmp, fmt.Sprintf("pokeclaude-%d.sock", os.Getuid()))
}

// IsDaemonRunning reports whether something is already listening on the socket.
// It dials with a short timeout; a successful connection means a live daemon.
func IsDaemonRunning(path string) bool {
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// prepareSocket cleans up a stale socket file before listening. It returns an
// error if a live daemon already owns the socket.
func prepareSocket(path string) error {
	if _, err := os.Stat(path); err == nil {
		if IsDaemonRunning(path) {
			return fmt.Errorf("another pokeclaude daemon is already running at %s", path)
		}
		// stale socket: remove it
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing stale socket: %w", err)
		}
	}
	return nil
}
