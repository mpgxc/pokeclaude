// Command pokeclaude renders active Claude Code sessions as ASCII Pokémon
// wandering around zones in your terminal.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pokeclaude",
		Short: "PokéClaude — suas sessões do Claude Code como Pokémon no terminal",
		Long: "PokéClaude escuta os hooks do Claude Code e desenha cada sessão/agente\n" +
			"ativo como um Pokémon ASCII andando dentro de zonas, com um balão de\n" +
			"pensamento mostrando a tarefa em execução.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		hookCmd(),
		tuiCmd(),
		demoCmd(),
		installCmd(),
		uninstallCmd(),
		doctorCmd(),
	)
	return root
}
