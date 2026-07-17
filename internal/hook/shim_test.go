package hook

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mpgxc/pokeclaude/internal/protocol"
)

// withStdin runs fn with os.Stdin temporarily fed from in.
func withStdin(t *testing.T, in string, fn func()) {
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

// TestRunMalformedNoPanic verifies the shim tolerates garbage on stdin without
// panicking. There is no daemon, so post() fails silently.
func TestRunMalformedNoPanic(t *testing.T) {
	inputs := []string{
		``,
		`not json at all`,
		`{"session_id":`,
		`{"session_id":"s","hook_event_name":"PreToolUse","tool_input":{"file_path":"x"}}`,
		`[]`,
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on input %q: %v", in, r)
				}
			}()
			withStdin(t, in, Run)
		}()
	}
}

func TestTolerantUnmarshalKeepsRaw(t *testing.T) {
	// unknown fields must be ignored, known fields decoded
	raw := `{"session_id":"abc","hook_event_name":"Stop","unknown_field":42}`
	var ev protocol.HookEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.SessionID != "abc" || ev.HookEventName != "Stop" {
		t.Errorf("decoded = %+v", ev)
	}
}
