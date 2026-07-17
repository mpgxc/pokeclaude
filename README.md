# PokéClaude

Suas sessões do **Claude Code** viram Pokémon ASCII andando pelo terminal. Cada
sessão (ou subagente) é um Pokémon dentro de uma *zona* — uma caixa por diretório
de trabalho — com um balão de pensamento mostrando a tarefa em execução.

```
╔══════════════════════════════════════════════════════════════╗
║ PokéClaude  2 agentes · 2 zonas                     08:50:39  ║
╠══════════════════════════════════════════════════════════════╣
║┌─ buni-ms-recharge ─────────┐┌─ buni-ms-boletos ────────────┐║
║│         ^(u_u)             ││ ╭───────────────────────╮    │║
║│          (~V~)             │││ 💭 adicionar retry no │      │║
║│           m m              │││ cons…                 │      │║
║│  ╭────────╯──────╮         ││╰────╮──────────────────╯      │║
║│  │ ⚡ npm test -- │         ││   (^w^)                       │║
║│  │ boletos.spec  │         ││   /|z|\                       │║
║│  ╰───────────────╯         ││    ' '                        │║
║│                            ││~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~│║
║└────────────────────────────┘└──────────────────────────────┘║
╠══════════════════════════════════════════════════════════════╣
║ bulbasaur    Working   Bash: npm test -- boletos.spec        ║
║ pikachu      Thinking  "adicionar retry no cons…"            ║
╚══════════════════════════════════════════════════════════════╝
```

## Como funciona

```
Claude Code ──hook(stdin JSON)──▶ pokeclaude hook ──unix socket──▶ pokeclaude tui
                                                                    │
                                              ingest → world → render (20fps)
```

- **`pokeclaude hook`** — shim ultraleve. O Claude Code o invoca em cada hook;
  ele lê o JSON do stdin, empacota num *envelope* e envia ao daemon via unix
  socket. Timeout de 50ms, sem retry, **sempre exit 0** — nunca atrapalha o
  Claude Code, mesmo com o daemon desligado.
- **`pokeclaude tui`** — daemon + interface. Sobe o socket server, mantém o
  mundo em memória, roda o game loop e desenha.

## Instalação

```sh
go build -o pokeclaude ./cmd/pokeclaude
sudo mv pokeclaude /usr/local/bin/      # ou qualquer lugar no PATH

pokeclaude install                       # injeta os hooks em ~/.claude/settings.json
pokeclaude doctor                        # valida socket, settings e terminal
```

Depois, num terminal dedicado:

```sh
pokeclaude tui
```

e use o Claude Code normalmente em outro terminal — os Pokémon aparecem sozinhos.

## Comandos

| Comando | Função |
|---|---|
| `pokeclaude tui` | sobe daemon + interface (foreground) |
| `pokeclaude demo` | roda a interface com eventos sintéticos, sem Claude Code |
| `pokeclaude hook` | shim chamado pelos hooks (uso interno) |
| `pokeclaude install` | injeta os hooks em `~/.claude/settings.json` (com backup) |
| `pokeclaude uninstall` | remove os hooks, preservando hooks de terceiros |
| `pokeclaude doctor` | valida socket, settings e tamanho do terminal |

### Teclas

`q`/`Ctrl+C` sai · `p` pausa · `Tab` alterna páginas de zonas.

## Experimente sem Claude Code

```sh
pokeclaude demo
```

Gera sessões falsas que nascem, pensam, usam ferramentas, chamam subagentes,
erram e terminam — ótimo para ver tudo em movimento.

## Estados

| Estado | Comportamento |
|---|---|
| Idle | perambula aleatoriamente pela zona |
| Thinking | parado, balão `...` pulsando (ou o prompt do usuário) |
| Working | pulinho vertical, balão com a ferramenta em uso |
| Error | tremida horizontal, balão 💥 |
| Done | feliz, some após 10s |

O Pokémon de cada sessão é **determinístico**: o mesmo `session_id` sempre vira
a mesma espécie. Subagentes (ferramenta `Task`) aparecem como sprites reduzidos
na mesma zona do pai.

## Arquitetura

```
cmd/pokeclaude/       # cobra: hook, tui, demo, install, uninstall, doctor
internal/protocol/    # tipos compartilhados hook ↔ daemon (HookEvent, Envelope)
internal/hook/        # shim: stdin → Envelope → POST socket
internal/ingest/      # unix socket http server → chan Envelope
internal/world/       # mundo: zonas, agentes, máquina de estados, reducer, movimento
internal/dex/         # espécies + sprites embutidos (embed.FS)
internal/render/      # framebuffer, sprites, balões, HUD, modo compacto
internal/tui/         # bubbletea: game loop 20fps
assets/sprites/       # sprites ASCII originais (.txt)
```

Detalhes de design em [`PLAN.md`](PLAN.md).

### Decisões

- **Sem persistência.** Fechou o TUI, perdeu o estado. É um brinquedo.
- **Sem TCP.** Unix socket apenas (`$XDG_RUNTIME_DIR/pokeclaude.sock`, com
  fallback em `$TMPDIR`). Windows fica fora por enquanto.
- **Sprites originais em ASCII** — nada de arte copiada. Emoji só nos balões, com
  largura tratada por `go-runewidth` (célula fantasma para runas de 2 colunas).
- Terminal abaixo de **80×24** cai num **modo compacto** (só a lista, sem mapa).

## Desenvolvimento

```sh
go test ./...            # testes
go test -race ./...      # com detector de corrida
go run ./cmd/pokeclaude demo
```

Cobertura de testes: reducer (evento → estado), bin-packing de zonas, geração de
balões (largura, quebra, cauda), alinhamento de runas largas, determinismo do
dex, tolerância do shim a JSON malformado e o transporte shim→daemon
fim-a-fim.

## Requisitos

Go 1.24+ · um terminal com suporte a UTF-8 e cores.
