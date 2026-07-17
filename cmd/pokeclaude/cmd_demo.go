package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mpgxc/pokeclaude/internal/protocol"
	"github.com/mpgxc/pokeclaude/internal/tui"
	"github.com/spf13/cobra"
)

func demoCmd() *cobra.Command {
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "roda a interface com eventos sintéticos (sem Claude Code)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ch := make(chan protocol.Envelope, 64)
			go generateDemo(ch, interval)
			p := tea.NewProgram(tui.New(ch), tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 1500*time.Millisecond, "intervalo entre eventos sintéticos")
	return cmd
}

// demoSession is a scripted fake session for the demo.
type demoSession struct {
	id    string
	cwd   string
	step  int
	tools []demoTool
}

type demoTool struct {
	name  string
	input map[string]string
}

func generateDemo(ch chan<- protocol.Envelope, interval time.Duration) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	cwds := []string{
		"/home/dev/buni-ms-boletos",
		"/home/dev/buni-ms-recharge",
		"/home/dev/pokeclaude",
	}
	tools := []demoTool{
		{"Read", map[string]string{"file_path": "internal/world/reducer.go"}},
		{"Grep", map[string]string{"pattern": "func.*Apply"}},
		{"Edit", map[string]string{"file_path": "consumer.go"}},
		{"Bash", map[string]string{"command": "go test ./... -run Boletos"}},
		{"Bash", map[string]string{"command": "npm test -- boletos.spec"}},
		{"Task", map[string]string{"description": "revisar diff"}},
		{"WebSearch", map[string]string{"query": "retry backoff kafka"}},
		{"TodoWrite", map[string]string{}},
	}

	var sessions []*demoSession
	emit := func(s *demoSession, name string, tool *demoTool, prompt string) {
		ev := protocol.HookEvent{
			SessionID:     s.id,
			CWD:           s.cwd,
			HookEventName: name,
			Prompt:        prompt,
		}
		if tool != nil {
			ev.ToolName = tool.name
			ev.ToolInput, _ = json.Marshal(tool.input)
		}
		if name == protocol.HookPostToolUse && rng.Intn(6) == 0 {
			ev.ToolResponse = json.RawMessage(`{"is_error":true}`)
		}
		ch <- protocol.Envelope{ReceivedAt: time.Now(), Event: ev}
	}

	nextID := 0
	for {
		// occasionally spawn a new session (up to 4)
		if len(sessions) < 4 && (len(sessions) == 0 || rng.Intn(4) == 0) {
			nextID++
			s := &demoSession{
				id:  fmt.Sprintf("demo-session-%d", nextID),
				cwd: cwds[rng.Intn(len(cwds))],
			}
			sessions = append(sessions, s)
			emit(s, protocol.HookSessionStart, nil, "")
			time.Sleep(interval)
			continue
		}

		s := sessions[rng.Intn(len(sessions))]
		switch s.step % 4 {
		case 0:
			emit(s, protocol.HookUserPromptSubmit, nil, "adicionar retry no consumer de boletos")
		case 1:
			t := tools[rng.Intn(len(tools))]
			emit(s, protocol.HookPreToolUse, &t, "")
		case 2:
			emit(s, protocol.HookPostToolUse, nil, "")
		case 3:
			if rng.Intn(3) == 0 {
				emit(s, protocol.HookStop, nil, "")
				// drop the session from rotation
				sessions = removeSession(sessions, s)
			} else {
				t := tools[rng.Intn(len(tools))]
				emit(s, protocol.HookPreToolUse, &t, "")
			}
		}
		s.step++
		time.Sleep(interval)
	}
}

func removeSession(list []*demoSession, s *demoSession) []*demoSession {
	out := list[:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}
