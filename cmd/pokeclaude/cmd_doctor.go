package main

import (
	"fmt"
	"os"

	"github.com/mpgxc/pokeclaude/internal/ingest"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "valida socket, settings e tamanho do terminal",
		RunE: func(cmd *cobra.Command, args []string) error {
			ok := true

			// socket / daemon
			path := ingest.SocketPath()
			fmt.Printf("socket:   %s\n", path)
			if ingest.IsDaemonRunning(path) {
				fmt.Println("  ✔ daemon rodando")
			} else {
				fmt.Println("  … daemon não está rodando (rode `pokeclaude tui`)")
			}

			// settings / hooks
			sp, err := settingsPath()
			if err != nil {
				return err
			}
			fmt.Printf("settings: %s\n", sp)
			m, err := readSettings(sp)
			if err != nil {
				fmt.Printf("  ✘ %v\n", err)
				ok = false
			} else {
				hooks, err := decodeHooks(m)
				if err != nil {
					fmt.Printf("  ✘ %v\n", err)
					ok = false
				} else {
					wired := 0
					for _, spec := range hookEventSpecs {
						for _, e := range hooks[spec.Name] {
							if entryHasMarker(e) {
								wired++
								break
							}
						}
					}
					if wired == len(hookEventSpecs) {
						fmt.Printf("  ✔ hooks instalados (%d/%d)\n", wired, len(hookEventSpecs))
					} else if wired == 0 {
						fmt.Println("  ✘ hooks não instalados (rode `pokeclaude install`)")
						ok = false
					} else {
						fmt.Printf("  ⚠ hooks parciais (%d/%d) — rode `pokeclaude install`\n", wired, len(hookEventSpecs))
					}
				}
			}

			// terminal size
			if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				fmt.Printf("terminal: %d×%d\n", w, h)
				if w < 80 || h < 24 {
					fmt.Println("  ⚠ menor que 80×24 — o modo compacto será usado")
				} else {
					fmt.Println("  ✔ tamanho adequado")
				}
			} else {
				fmt.Println("terminal: não foi possível detectar o tamanho (não é um TTY?)")
			}

			if !ok {
				return fmt.Errorf("problemas encontrados — veja acima")
			}
			fmt.Println("\ntudo certo! ✨")
			return nil
		},
	}
}
