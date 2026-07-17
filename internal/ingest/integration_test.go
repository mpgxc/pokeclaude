package ingest_test

import (
	"os"
	"testing"
	"time"

	"github.com/mpgxc/pokeclaude/internal/hook"
	"github.com/mpgxc/pokeclaude/internal/ingest"
)

// TestShimToDaemon exercises the full transport: the shim reads stdin, posts to
// the unix socket, and the daemon delivers the envelope on its Events channel.
func TestShimToDaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	path := ingest.SocketPath()

	srv, err := ingest.NewServer(path, 16, "")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve() }()

	// give the listener a moment to accept
	if !waitDaemon(path) {
		t.Fatal("daemon did not come up")
	}

	const payload = `{"session_id":"sess-1","cwd":"/home/dev/proj","hook_event_name":"SessionStart"}`
	feedStdin(t, payload, hook.Run)

	select {
	case env := <-srv.Events():
		if env.Event.SessionID != "sess-1" {
			t.Errorf("session id = %q, want sess-1", env.Event.SessionID)
		}
		if env.Event.HookEventName != "SessionStart" {
			t.Errorf("event = %q, want SessionStart", env.Event.HookEventName)
		}
		if len(env.Raw) == 0 {
			t.Error("raw payload not preserved")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestDoubleDaemonRejected(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	path := ingest.SocketPath()

	srv, err := ingest.NewServer(path, 4, "")
	if err != nil {
		t.Fatalf("first NewServer: %v", err)
	}
	defer srv.Close()
	go func() { _ = srv.Serve() }()
	waitDaemon(path)

	if _, err := ingest.NewServer(path, 4, ""); err == nil {
		t.Error("second daemon should be rejected while first is running")
	}
}

func waitDaemon(path string) bool {
	for i := 0; i < 50; i++ {
		if ingest.IsDaemonRunning(path) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func feedStdin(t *testing.T, in string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig; _ = r.Close() }()
	go func() {
		_, _ = w.WriteString(in)
		_ = w.Close()
	}()
	fn()
}
