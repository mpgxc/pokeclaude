package arena

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ControlSocketPath returns the unix socket the arena listens on for control
// commands, preferring $XDG_RUNTIME_DIR with a $TMPDIR fallback.
func ControlSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "pokeclaude-arena.sock")
	}
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	return filepath.Join(tmp, fmt.Sprintf("pokeclaude-arena-%d.sock", os.Getuid()))
}

// Command is a single control instruction for one side.
type Command struct {
	Side   string `json:"side"`   // "A" or "B"
	Action string `json:"action"` // e.g. "leve", "esquiva", "especial"
}

// ControlServer receives commands over a unix socket and routes them to the
// side's RemoteController. It lets a live AI agent (or the CLI) steer a fighter.
type ControlServer struct {
	path string
	ln   net.Listener
	srv  *http.Server
	a, b *RemoteController
}

// NewControlServer binds a control server, wiring sides A and B to their
// controllers.
func NewControlServer(path string, a, b *RemoteController) (*ControlServer, error) {
	if _, err := os.Stat(path); err == nil {
		// remove a stale socket (best effort)
		_ = os.Remove(path)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	s := &ControlServer{path: path, ln: ln, a: a, b: b}
	mux := http.NewServeMux()
	mux.HandleFunc("/cmd", s.handleCmd)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	s.srv = &http.Server{Handler: mux}
	return s, nil
}

// Serve runs until Close.
func (s *ControlServer) Serve() error {
	err := s.srv.Serve(s.ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Close shuts the server down and removes the socket.
func (s *ControlServer) Close() error {
	_ = s.srv.Close()
	err := s.ln.Close()
	_ = os.Remove(s.path)
	return err
}

func (s *ControlServer) handleCmd(w http.ResponseWriter, r *http.Request) {
	side := r.URL.Query().Get("side")
	action := r.URL.Query().Get("action")
	if side == "" && action == "" {
		var c Command
		if err := json.NewDecoder(r.Body).Decode(&c); err == nil {
			side, action = c.Side, c.Action
		}
	}
	ctrl := s.controllerFor(side)
	if ctrl == nil {
		http.Error(w, "lado inválido (use A ou B)", http.StatusBadRequest)
		return
	}
	act, ok := ParseAction(action)
	if !ok {
		http.Error(w, "ação desconhecida: "+action, http.StatusBadRequest)
		return
	}
	ctrl.Set(act)
	w.WriteHeader(http.StatusAccepted)
}

func (s *ControlServer) controllerFor(side string) *RemoteController {
	switch side {
	case "A", "a", "0":
		return s.a
	case "B", "b", "1":
		return s.b
	default:
		return nil
	}
}

// SendCommand posts a control command to a running arena over the socket.
func SendCommand(path, side, action string) error {
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			},
		},
	}
	url := fmt.Sprintf("http://arena/cmd?side=%s&action=%s", side, action)
	resp, err := client.Post(url, "text/plain", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("arena recusou o comando (status %d)", resp.StatusCode)
	}
	return nil
}
