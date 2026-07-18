package arena

import (
	"path/filepath"
	"testing"
	"time"
)

func TestControlServerRoutesCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arena.sock")
	ra, rb := NewRemote("A"), NewRemote("B")
	srv, err := NewControlServer(path, ra, rb)
	if err != nil {
		t.Fatalf("NewControlServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve() }()

	// wait for the listener
	up := false
	for i := 0; i < 50; i++ {
		if err := SendCommand(path, "A", "leve"); err == nil {
			up = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !up {
		t.Fatal("server never accepted a command")
	}

	if got := ra.Decide(FighterView{}, FighterView{}); got.Action != ActLight {
		t.Errorf("side A action = %v, want ActLight", got.Action)
	}

	if err := SendCommand(path, "B", "esquiva"); err != nil {
		t.Fatalf("SendCommand B: %v", err)
	}
	if got := rb.Decide(FighterView{}, FighterView{}); got.Action != ActDodge {
		t.Errorf("side B action = %v, want ActDodge", got.Action)
	}
}

func TestControlServerRejectsBadInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arena.sock")
	ra, rb := NewRemote("A"), NewRemote("B")
	srv, err := NewControlServer(path, ra, rb)
	if err != nil {
		t.Fatalf("NewControlServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve() }()
	time.Sleep(60 * time.Millisecond)

	if err := SendCommand(path, "Z", "leve"); err == nil {
		t.Error("expected error for invalid side")
	}
	if err := SendCommand(path, "A", "banana"); err == nil {
		t.Error("expected error for invalid action")
	}
}
