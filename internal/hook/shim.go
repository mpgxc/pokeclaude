// Package hook implements the ultralight shim invoked by Claude Code on every
// hook. It reads the event JSON from stdin, wraps it and POSTs it to the daemon
// over the unix socket. It must be fast, must never print to stdout and must
// always exit 0 so it never disrupts Claude Code.
package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/mpgxc/pokeclaude/internal/ingest"
	"github.com/mpgxc/pokeclaude/internal/protocol"
)

const (
	maxStdin    = 1 << 20 // 1 MiB
	postTimeout = 50 * time.Millisecond
)

// Run executes the shim. It never returns an error to the caller: any failure
// is swallowed (optionally logged to stderr when POKECLAUDE_DEBUG=1) so the
// process can exit 0 unconditionally.
func Run() {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdin))
	if err != nil {
		debugf("read stdin: %v", err)
		return
	}

	var ev protocol.HookEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		// tolerant: proceed with only the raw payload
		debugf("unmarshal: %v", err)
	}

	env := protocol.Envelope{
		ReceivedAt: time.Now(),
		PID:        os.Getpid(),
		Event:      ev,
		Raw:        json.RawMessage(raw),
	}
	body, err := json.Marshal(env)
	if err != nil {
		debugf("marshal: %v", err)
		return
	}

	if err := post(body); err != nil {
		debugf("post: %v", err)
	}
}

// post sends the envelope to the daemon over the unix socket. Errors (including
// "no daemon running") are returned but callers ignore them.
func post(body []byte) error {
	path := ingest.SocketPath()
	client := &http.Client{
		Timeout: postTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			},
		},
	}
	// The host in the URL is ignored for unix sockets.
	req, err := http.NewRequest(http.MethodPost, "http://pokeclaude/event", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

// debugf writes to stderr only when POKECLAUDE_DEBUG=1.
func debugf(format string, args ...any) {
	if os.Getenv("POKECLAUDE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "pokeclaude hook: "+format+"\n", args...)
	}
}
