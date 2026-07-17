package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// resolveBinary returns the absolute path of the running executable, for use in
// the hook command string.
func resolveBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exe, nil
}

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "injeta os hooks do PokéClaude em ~/.claude/settings.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := resolveBinary()
			if err != nil {
				return err
			}
			command := exe + " hook"

			path, err := settingsPath()
			if err != nil {
				return err
			}
			m, err := readSettings(path)
			if err != nil {
				return err
			}
			bak, err := backupSettings(path)
			if err != nil {
				return err
			}

			hooks, err := decodeHooks(m)
			if err != nil {
				return err
			}
			added := installHooks(hooks, command)
			if err := encodeHooks(m, hooks); err != nil {
				return err
			}
			if err := writeSettings(path, m); err != nil {
				return err
			}

			if bak != "" {
				fmt.Printf("backup salvo em %s\n", bak)
			}
			if added == 0 {
				fmt.Println("hooks já estavam instalados — nada a fazer.")
			} else {
				fmt.Printf("hooks instalados (%d eventos) em %s\n", added, path)
			}
			fmt.Printf("comando: %s\n", command)
			fmt.Println("agora rode `pokeclaude tui` em outro terminal.")
			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "remove os hooks do PokéClaude de ~/.claude/settings.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := settingsPath()
			if err != nil {
				return err
			}
			m, err := readSettings(path)
			if err != nil {
				return err
			}
			bak, err := backupSettings(path)
			if err != nil {
				return err
			}
			hooks, err := decodeHooks(m)
			if err != nil {
				return err
			}
			removed := uninstallHooks(hooks)
			if err := encodeHooks(m, hooks); err != nil {
				return err
			}
			if err := writeSettings(path, m); err != nil {
				return err
			}
			if bak != "" {
				fmt.Printf("backup salvo em %s\n", bak)
			}
			if removed == 0 {
				fmt.Println("nenhum hook do PokéClaude encontrado.")
			} else {
				fmt.Printf("hooks removidos (%d entradas).\n", removed)
			}
			return nil
		},
	}
}
