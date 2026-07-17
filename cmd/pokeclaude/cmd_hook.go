package main

import (
	"github.com/mpgxc/pokeclaude/internal/hook"
	"github.com/spf13/cobra"
)

func hookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "hook",
		Short:  "shim chamado pelos hooks do Claude Code (lê JSON do stdin)",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			// Never fail: hook.Run swallows all errors and we always exit 0.
			hook.Run()
		},
	}
}
