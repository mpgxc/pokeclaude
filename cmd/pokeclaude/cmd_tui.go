package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mpgxc/pokeclaude/internal/ingest"
	"github.com/mpgxc/pokeclaude/internal/tui"
	"github.com/spf13/cobra"
)

func tuiCmd() *cobra.Command {
	var debug bool
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "sobe o daemon e a interface (foreground)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ingest.SocketPath()
			debugPath := ""
			if debug {
				debugPath = "/tmp/pokeclaude-events.jsonl"
			}
			srv, err := ingest.NewServer(path, 256, debugPath)
			if err != nil {
				return err
			}
			defer srv.Close()

			go func() {
				if err := srv.Serve(); err != nil {
					fmt.Println("erro no servidor:", err)
				}
			}()

			p := tea.NewProgram(tui.New(srv.Events()), tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
	cmd.Flags().BoolVar(&debug, "debug", false, "grava os eventos crus em /tmp/pokeclaude-events.jsonl")
	return cmd
}
