package ingest

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/mpgxc/pokeclaude/internal/protocol"
)

// maxBody caps the size of a single event payload.
const maxBody = 1 << 20 // 1 MiB

// Server accepts hook envelopes over a unix socket and forwards them on Events.
type Server struct {
	path     string
	listener net.Listener
	httpSrv  *http.Server
	events   chan protocol.Envelope
	debug    *debugSink
}

// NewServer creates (but does not start) a server bound to path. The events
// channel is buffered; when full, incoming events are dropped with a log line
// so the daemon never blocks the shim.
func NewServer(path string, bufferCap int, debugPath string) (*Server, error) {
	if err := prepareSocket(path); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	s := &Server{
		path:     path,
		listener: ln,
		events:   make(chan protocol.Envelope, bufferCap),
	}
	if debugPath != "" {
		s.debug = newDebugSink(debugPath)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/event", s.handleEvent)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	s.httpSrv = &http.Server{Handler: mux}
	return s, nil
}

// Events returns the channel of received envelopes.
func (s *Server) Events() <-chan protocol.Envelope { return s.events }

// Path returns the socket path in use.
func (s *Server) Path() string { return s.path }

// Serve runs the HTTP server over the unix listener. It blocks until Close.
func (s *Server) Serve() error {
	err := s.httpSrv.Serve(s.listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Close shuts down the server and removes the socket file.
func (s *Server) Close() error {
	_ = s.httpSrv.Close()
	err := s.listener.Close()
	_ = os.Remove(s.path)
	if s.debug != nil {
		s.debug.Close()
	}
	return err
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var env protocol.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		// tolerate: keep the raw bytes for debugging, still ack
		env = protocol.Envelope{Raw: body}
	}
	if s.debug != nil {
		s.debug.Write(body)
	}
	select {
	case s.events <- env:
	default:
		log.Printf("pokeclaude: event buffer full, dropping %s", env.Event.HookEventName)
	}
	w.WriteHeader(http.StatusAccepted)
}
