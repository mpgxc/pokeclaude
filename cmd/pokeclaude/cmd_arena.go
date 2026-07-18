package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mpgxc/pokeclaude/internal/arena"
	"github.com/mpgxc/pokeclaude/internal/dex"
	"github.com/spf13/cobra"
)

func arenaCmd() *cobra.Command {
	var (
		aName, bName string
		seed         int64
		rounds       int
		control      string
		headless     bool
		slow         bool
	)
	cmd := &cobra.Command{
		Use:   "arena",
		Short: "rinha estilo Mortal Kombat entre dois pets (modo game)",
		Long: "Arena: uma luta 1v1 entre dois Pokémon-galos, com ataque, defesa,\n" +
			"esquiva, combos, vida, mana e finalização. Controláveis por bots ou\n" +
			"por agentes de IA via `pokeclaude arena-cmd`. O render gráfico (raylib)\n" +
			"exige compilar com `-tags raylib`; sem isso, roda em modo headless.",
		RunE: func(cmd *cobra.Command, args []string) error {
			all := dex.All()
			spA := pickSpecies(aName, all[0])
			spB := pickSpecies(bName, all[1%len(all)])

			cfg := arena.Config{Seed: seed, RoundsToWin: rounds}
			remote := control == "remote"

			var ctrlA, ctrlB arena.Controller
			var server *arena.ControlServer
			if remote {
				// no round timer: the fighters idle and wait for commands,
				// so the match only ends on a K.O., not by itself.
				cfg.RoundSeconds = -1
				ra, rb := arena.NewRemote("A"), arena.NewRemote("B")
				ctrlA, ctrlB = ra, rb
				srv, err := arena.NewControlServer(arena.ControlSocketPath(), ra, rb)
				if err != nil {
					return err
				}
				server = srv
				go func() { _ = srv.Serve() }()
				defer srv.Close()
				fmt.Printf("controle remoto ativo — socket: %s\n", arena.ControlSocketPath())
				fmt.Println("os lutadores aguardam comandos. Ex.:")
				fmt.Println("  pokeclaude arena-cmd --side A --action avancar")
				fmt.Println("  pokeclaude arena-cmd --side A --action leve")
				fmt.Println("  pokeclaude arena-cmd --side B --action esquiva")
			} else {
				ctrlA = arena.NewBot(seed+1, 0.55)
				ctrlB = arena.NewBot(seed+2, 0.55)
			}

			m := arena.Build(cfg, spA, spB, ctrlA, ctrlB)

			if graphicsBuilt && !headless {
				return runGraphics(m, server)
			}
			if !graphicsBuilt && !headless {
				fmt.Println("(sem raylib — rodando em modo texto; para a janela gráfica, compile com `-tags raylib`)")
			}
			// remote mode must run in real time so commands can arrive; the bot
			// demo can fast-forward unless --slow is set.
			runHeadlessArena(m, slow || remote, os.Stdout)
			return nil
		},
	}
	cmd.Flags().StringVar(&aName, "a", "", "espécie/nome do lutador A (vazio = padrão)")
	cmd.Flags().StringVar(&bName, "b", "", "espécie/nome do lutador B (vazio = padrão)")
	cmd.Flags().Int64Var(&seed, "seed", 1, "semente determinística")
	cmd.Flags().IntVar(&rounds, "rounds", 2, "rounds para vencer (melhor de 2N-1)")
	cmd.Flags().StringVar(&control, "control", "bot", "controle: bot | remote")
	cmd.Flags().BoolVar(&headless, "headless", false, "força o modo texto (sem janela gráfica)")
	cmd.Flags().BoolVar(&slow, "slow", false, "ritmo lento no headless (para acompanhar a luta)")
	return cmd
}

func arenaCmdCmd() *cobra.Command {
	var side, action string
	cmd := &cobra.Command{
		Use:   "arena-cmd",
		Short: "envia um comando de controle para uma arena em execução",
		Long: "Injeta uma ação para o lado A ou B de uma arena rodando com --control remote.\n" +
			"Ações: avancar, recuar, leve, pesado, especial, bloquear, esquiva, super.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := arena.ParseAction(action); !ok {
				return fmt.Errorf("ação desconhecida: %q", action)
			}
			if err := arena.SendCommand(arena.ControlSocketPath(), side, action); err != nil {
				return err
			}
			fmt.Printf("ok: %s ← %s\n", side, action)
			return nil
		},
	}
	cmd.Flags().StringVar(&side, "side", "A", "lado a controlar: A ou B")
	cmd.Flags().StringVar(&action, "action", "", "ação (avancar, leve, pesado, especial, bloquear, esquiva, super)")
	_ = cmd.MarkFlagRequired("action")
	return cmd
}

// pickSpecies resolves a name to a species: exact match, else a deterministic
// hash of the name, else the fallback for an empty name.
func pickSpecies(name string, fallback dex.Species) dex.Species {
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	for _, sp := range dex.All() {
		if strings.EqualFold(sp.Name, name) {
			return sp
		}
	}
	return dex.SpeciesFor(name)
}

// runHeadlessArena simulates the match and streams the fight log to out. When
// realtime is set it paces the simulation to wall-clock time (so remote control
// commands can arrive and the fight can be followed live).
func runHeadlessArena(m *arena.Arena, realtime bool, out io.Writer) {
	fmt.Fprintln(out, vsBanner(m))
	const frame = 1.0 / 60.0
	printed := 0
	for !m.Over() {
		m.Advance(frame)
		log := m.Log()
		for ; printed < len(log); printed++ {
			fmt.Fprintln(out, "  • "+log[printed])
		}
		if realtime {
			time.Sleep(time.Second / 60)
		}
	}
	sa, sb := m.Score()
	fmt.Fprintf(out, "\n🏆 PLACAR FINAL: %s %d — %d %s\n", m.A.Nick, sa, sb, m.B.Nick)
}

func vsBanner(m *arena.Arena) string {
	var s strings.Builder
	fmt.Fprintf(&s, "⚔  RINHA — %s  VS  %s\n", strings.ToUpper(m.A.Nick), strings.ToUpper(m.B.Nick))
	fmt.Fprintln(&s, statsLine(m.A))
	fmt.Fprintln(&s, statsLine(m.B))
	return s.String()
}

func statsLine(f *arena.Fighter) string {
	return fmt.Sprintf("   %-12s ATK %.0f · DEF %.0f · VEL %.0f · CMB %.0f · HP %.0f · MANA %.0f",
		f.Nick, f.Stats.Attack, f.Stats.Defense, f.Stats.Speed, f.Stats.Combo, f.Stats.MaxHP, f.Stats.MaxMana)
}
