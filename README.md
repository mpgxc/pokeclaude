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

## Instalação rápida

Requer **Go 1.25+** e um terminal com UTF-8 e cores.

```sh
git clone https://github.com/mpgxc/pokeclaude.git
cd pokeclaude

make install     # compila, instala em /usr/local/bin e injeta os hooks
pokeclaude doctor
```

Depois, num terminal dedicado:

```sh
pokeclaude tui   # e use o Claude Code normalmente em outro terminal
```

Sem `make`? Faça na mão:

```sh
go build -o pokeclaude ./cmd/pokeclaude
sudo mv pokeclaude /usr/local/bin/
pokeclaude install
```

Atalhos do `make` (rode `make help` para a lista): `build`, `install`,
`uninstall`, `tui`, `demo`, `arena`, `build-gfx` (Arena gráfica), `test`,
`race`, `check`, `clean`. Para remover tudo: `make uninstall`.

> Ambientes **headless/remotos** não abrem a TUI nem a janela gráfica (faltam
> terminal/display) — rode numa máquina com terminal real.

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
| `pokeclaude arena` | **modo game**: rinha 1v1 estilo Mortal Kombat entre dois pets |
| `pokeclaude arena-cmd` | envia um comando de controle para uma arena em execução |

### Teclas

`q`/`Ctrl+C` sai · `p` pausa · `Tab` alterna páginas de zonas · `m` alterna
o **modo de movimentação** · `h` (ou `Espaço`) dispara a **Hiper Velocidade**
no Space Drift.

## Experimente sem Claude Code

```sh
pokeclaude demo                 # modo padrão (zonas)
pokeclaude demo --mode space    # já começa no Space Drift
```

Gera sessões falsas que nascem, pensam, usam ferramentas, chamam subagentes,
erram e terminam — ótimo para ver tudo em movimento.

## Modos de movimentação

Aperte `m` para alternar entre os modos (ou use `--mode` ao iniciar).

### Zonas (padrão)

Cada Pokémon caminha dentro da zona do seu diretório de trabalho, com balão de
pensamento mostrando a tarefa.

### Space Drift 🚀

Em vez de caminhar, os Pokémon pilotam naves espaciais e derivam pelo universo:

- **Fundo com efeito de velocidade** — o campo estelar rola criando linhas de
  movimento; acelera quando alguma nave está trabalhando (impulso).
- **Obstáculos** — asteroides `(@)`/`o` e cometas `*` cruzam o mapa; as naves
  **desviam** automaticamente do que estiver na rota.
- **Tiros** — as naves **disparam lasers** contra asteroides e cometas à frente,
  destruindo-os com explosões.
- **Colisões** — bater num obstáculo sacode a nave (flash vermelho) e destrói o
  obstáculo.
- **Dobra Espacial (Hiper Velocidade)** — aperte `h`/`Espaço`: todas as naves
  dão um **salto quântico** para outra região do mapa, com um efeito de
  **distorção espaço-temporal** (o campo estelar se estica em túnel e cada nave
  deixa um rastro de dobra `=`).

O estado do agente modula a nave: **Working** = impulso (mais rápido, com chama
de propulsor), **Thinking** = deriva lenta, **Error** = dano/tremida,
**Idle** = cruzeiro.

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

## Modo Game — Arena 🥊

Uma **rinha 1v1 estilo Mortal Kombat** entre dois pets-galos. Cada lutador tem
**vida (HP)**, **mana**, **vidas/rounds** (melhor de 3) e stats derivados da
espécie: **ataque, defesa, esquiva/velocidade, combo**. As ações — **soco leve,
chute pesado, especial** (gasta mana), **bloqueio**, **esquiva** (i-frames) e
**FINALIZAÇÃO** (super, com barra cheia) — têm *frame data* (startup/active/
recovery), então dá pra encaixar **combos** de verdade.

```sh
# rinha bot vs bot, roda em texto (headless) em qualquer lugar
pokeclaude arena --seed 3
pokeclaude arena --a pikachu --b charmander --slow

# render gráfico (raylib) — exige compilar com a tag e um display
go build -tags raylib -o pokeclaude ./cmd/pokeclaude
./pokeclaude arena
```

### Controle por agentes de IA (IA × IA)

A ideia central: **um agente de IA controla o galo** injetando comandos. Suba a
arena com controle remoto e comande cada lado por um socket, a uma cadência de
~250ms (um LLM não reage a 60fps):

```sh
pokeclaude arena --control remote          # abre o socket de controle
pokeclaude arena-cmd --side A --action leve
pokeclaude arena-cmd --side B --action esquiva
```

Ações: `avancar`, `recuar`, `leve`, `pesado`, `especial`, `bloquear`,
`esquiva`, `super`. Um agente lê o estado do jogo e decide a próxima ação —
duas sessões de IA podem se enfrentar. Sem comandos, um **bot heurístico**
determinístico assume, o que mantém as simulações reproduzíveis (e testáveis).

> **Arte:** lutadores e efeitos são **procedurais e originais** (barras de vida
> estilo arcade, hit-sparks, screen shake, "ROUND 1 — LUTAR!", "K.O.",
> "VITÓRIA IMPECÁVEL"). Nada de arte copiada.
>
> **Build gráfico:** o renderer raylib fica atrás da build tag `raylib` e
> precisa de libs de sistema (OpenGL/X11/Wayland). O núcleo (engine) compila e
> roda **headless** sem nada disso. Não rode `go mod tidy` sem `-tags raylib`,
> senão a dependência do raylib é removida do `go.mod`.

## Arquitetura

```
cmd/pokeclaude/       # cobra: hook, tui, demo, install, uninstall, doctor
internal/protocol/    # tipos compartilhados hook ↔ daemon (HookEvent, Envelope)
internal/hook/        # shim: stdin → Envelope → POST socket
internal/ingest/      # unix socket http server → chan Envelope
internal/world/       # mundo: zonas, agentes, máquina de estados, reducer, movimento
                      #   mode.go + space.go = modo Space Drift (naves, física, warp)
internal/dex/         # espécies + sprites embutidos (embed.FS)
internal/render/      # framebuffer, sprites, balões, HUD, modo compacto
                      #   space.go = renderer do Space Drift (starfield, naves, warp)
internal/tui/         # bubbletea: game loop 20fps
internal/arena/       # modo game: engine de combate puro (determinístico, testável)
                      #   controllers (bot + remoto), socket de controle, snapshot
internal/arena/rl/    # renderer raylib da Arena (//go:build raylib)
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
dex, tolerância do shim a JSON malformado, o transporte shim→daemon fim-a-fim e
o Space Drift (física de naves, colisões laser/nave × obstáculo, warp e limites).

## Requisitos

Go 1.24+ · um terminal com suporte a UTF-8 e cores.
