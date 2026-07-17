package world

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mpgxc/pokeclaude/internal/protocol"
)

func env(name, session, cwd string, mut func(*protocol.HookEvent)) protocol.Envelope {
	e := protocol.HookEvent{SessionID: session, CWD: cwd, HookEventName: name}
	if mut != nil {
		mut(&e)
	}
	return protocol.Envelope{ReceivedAt: time.Unix(1000, 0), Event: e}
}

func newTestWorld() *World {
	return NewWithSeed(Rect{X: 1, Y: 3, W: 78, H: 14}, 42)
}

func TestReducerStateTransitions(t *testing.T) {
	const sid = "s1"
	const cwd = "/home/dev/proj"
	id := AgentID(sid)

	cases := []struct {
		name string
		ev   protocol.Envelope
		want State
	}{
		{"start", env(protocol.HookSessionStart, sid, cwd, nil), StateIdle},
		{"prompt", env(protocol.HookUserPromptSubmit, sid, cwd, func(e *protocol.HookEvent) {
			e.Prompt = "faça algo"
		}), StateThinking},
		{"pretool", env(protocol.HookPreToolUse, sid, cwd, func(e *protocol.HookEvent) {
			e.ToolName = "Bash"
			e.ToolInput = json.RawMessage(`{"command":"go test"}`)
		}), StateWorking},
		{"posttool_ok", env(protocol.HookPostToolUse, sid, cwd, nil), StateThinking},
		{"posttool_err", env(protocol.HookPostToolUse, sid, cwd, func(e *protocol.HookEvent) {
			e.ToolResponse = json.RawMessage(`{"is_error":true}`)
		}), StateError},
		{"stop", env(protocol.HookStop, sid, cwd, nil), StateDone},
	}

	w := newTestWorld()
	for _, c := range cases {
		w.Apply(c.ev)
		got, ok := w.AgentState(id)
		if !ok {
			t.Fatalf("%s: agent missing", c.name)
		}
		if got != c.want {
			t.Errorf("%s: state = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSessionStartCreatesZoneAndAgent(t *testing.T) {
	w := newTestWorld()
	w.Apply(env(protocol.HookSessionStart, "s1", "/a/b/proj", nil))
	if w.AgentCount() != 1 {
		t.Errorf("agents = %d, want 1", w.AgentCount())
	}
	if w.ZoneCount() != 1 {
		t.Errorf("zones = %d, want 1", w.ZoneCount())
	}
}

func TestLazySpawnOnUnknownSession(t *testing.T) {
	w := newTestWorld()
	// daemon started mid-session: a PreToolUse arrives with no prior SessionStart
	w.Apply(env(protocol.HookPreToolUse, "late", "/x/y", func(e *protocol.HookEvent) {
		e.ToolName = "Read"
		e.ToolInput = json.RawMessage(`{"file_path":"/x/y/main.go"}`)
	}))
	if st, ok := w.AgentState("late"); !ok || st != StateWorking {
		t.Errorf("lazy spawn failed: ok=%v state=%v", ok, st)
	}
}

func TestTaskSpawnsSubagent(t *testing.T) {
	w := newTestWorld()
	w.Apply(env(protocol.HookSessionStart, "root", "/p", nil))
	w.Apply(env(protocol.HookPreToolUse, "root", "/p", func(e *protocol.HookEvent) {
		e.ToolName = "Task"
		e.ToolInput = json.RawMessage(`{"description":"help"}`)
	}))
	if w.AgentCount() != 2 {
		t.Fatalf("agents = %d, want 2 (root + subagent)", w.AgentCount())
	}
	if _, ok := w.AgentState("root#1"); !ok {
		t.Error("subagent root#1 not found")
	}
}

func TestSessionEndFadesAgent(t *testing.T) {
	w := newTestWorld()
	w.Apply(env(protocol.HookSessionStart, "s", "/p", nil))
	w.Apply(env(protocol.HookSessionEnd, "s", "/p", nil))
	// after the fade window a tick should GC the agent
	future := time.Unix(1000, 0).Add(3 * time.Second)
	w.Tick(0.05, future)
	if _, ok := w.AgentState("s"); ok {
		t.Error("agent should have been removed after SessionEnd fade")
	}
}

func TestDeadSessionGC(t *testing.T) {
	w := newTestWorld()
	w.Apply(env(protocol.HookSessionStart, "s", "/p", nil))
	// no SessionEnd; jump past the dead-session TTL
	future := time.Unix(1000, 0).Add(16 * time.Minute)
	w.Tick(0.05, future)
	if _, ok := w.AgentState("s"); ok {
		t.Error("dead session should be garbage collected after TTL")
	}
}

func TestIsError(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{``, false},
		{`{"is_error":true}`, true},
		{`{"is_error":false}`, false},
		{`{"error":"boom"}`, true},
		{`{"status":"error"}`, true},
		{`{"ok":true}`, false},
		{`"plain string with Error inside"`, true},
	}
	for _, c := range cases {
		if got := isError(json.RawMessage(c.raw)); got != c.want {
			t.Errorf("isError(%s) = %v, want %v", c.raw, got, c.want)
		}
	}
}
