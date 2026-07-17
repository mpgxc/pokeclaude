// Package protocol defines the shared types exchanged between the hook shim
// and the daemon. Keeping them in one place lets both sides evolve together.
package protocol

import (
	"encoding/json"
	"time"
)

// HookEvent is the JSON payload Claude Code writes to a hook's stdin.
//
// The schema can drift between Claude Code versions, so unmarshalling MUST be
// tolerant: unknown fields are ignored and missing fields decode to their zero
// value. The daemon also keeps the raw bytes around (see Envelope.Raw) for
// debugging.
type HookEvent struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name,omitempty"`
	ToolInput      json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse   json.RawMessage `json:"tool_response,omitempty"`
	Prompt         string          `json:"prompt,omitempty"` // UserPromptSubmit
}

// Hook event names emitted by Claude Code.
const (
	HookSessionStart     = "SessionStart"
	HookUserPromptSubmit = "UserPromptSubmit"
	HookPreToolUse       = "PreToolUse"
	HookPostToolUse      = "PostToolUse"
	HookStop             = "Stop"
	HookSubagentStop     = "SubagentStop"
	HookSessionEnd       = "SessionEnd"
)

// Envelope wraps a HookEvent with metadata added by the shim before it is sent
// to the daemon over the unix socket.
type Envelope struct {
	ReceivedAt time.Time       `json:"received_at"`
	PID        int             `json:"pid"`
	Event      HookEvent       `json:"event"`
	Raw        json.RawMessage `json:"raw"`
}
