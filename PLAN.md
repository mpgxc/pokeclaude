# PokéClaude — Plano de Implementação

Ferramenta de terminal em Go que escuta os hooks do Claude Code e renderiza cada
sessão/agente ativo como um Pokémon ASCII andando dentro de zonas (caixas de
área, lagos, grama), com balão de pensamento mostrando a tarefa em execução.

---

## 1. Visão geral

Dois binários lógicos, um executável:

- `pokeclaude hook` — shim ultraleve. O Claude Code invoca em cada hook, lê o
  JSON do stdin, empacota e envia pro daemon via unix socket. **Sempre exit 0.**
- `pokeclaude tui` — daemon + TUI. Sobe o socket server, mantém o mundo em
  memória, roda o game loop e desenha.

```
Claude Code ──hook(stdin JSON)──> pokeclaude hook ──unix socket──> pokeclaude tui
                                                                    │
                                              ingest → world → render (20fps)
```

Comandos:

| Comando | Função |
|---|---|
| `pokeclaude tui` | sobe daemon + interface (foreground) |
| `pokeclaude hook` | shim chamado pelos hooks |
| `pokeclaude install` | injeta os hooks em `~/.claude/settings.json` |
| `pokeclaude uninstall` | remove os hooks |
| `pokeclaude doctor` | valida socket, settings, tamanho do terminal |

---

## 2. Stack

| Camada | Escolha | Motivo |
|---|---|---|
| CLI | `spf13/cobra` | subcomandos |
| TUI | `charmbracelet/bubbletea` | game loop + Msg é natural pra eventos |
| Estilo | `charmbracelet/lipgloss` | cor/borda por célula |
| Transporte | `net/http` sobre `net.UnixListener` | sem porta, sem conflito, permissão de fs |
| Sprites | `embed.FS` | binário único |
| Estado | in-memory, `sync.RWMutex` | não precisa persistir |

Go 1.24+. Zero dependência externa além de cobra/bubbletea/lipgloss/go-runewidth.

---

## 3. Layout de pacotes

```
cmd/pokeclaude/
  main.go              # cobra root
  cmd_hook.go
  cmd_tui.go
  cmd_demo.go
  cmd_install.go       # install + uninstall
  cmd_doctor.go
  settings.go          # merge não-destrutivo dos hooks
internal/protocol/     # tipos compartilhados hook <-> daemon
  event.go             # HookEvent (payload do Claude Code) + Envelope
internal/hook/
  shim.go              # stdin -> Envelope -> POST socket, timeout 50ms
internal/ingest/
  server.go            # unix socket http server -> chan protocol.Envelope
  socket.go            # path resolution, stale socket cleanup
  debug.go             # dump jsonl opcional
internal/world/
  world.go             # World: zones, agents, tick
  agent.go             # Agent, State machine
  zone.go              # Zone, alocação/bin-packing
  reducer.go           # Envelope -> mutação de estado
  movement.go          # wander, lerp, target picking
  thought.go           # geração de texto do balão a partir do tool_input
  geom.go              # Vec2, Rect
internal/dex/
  dex.go               # Species, lookup determinístico
  sprites.go           # embed + parse dos .txt
internal/render/
  frame.go             # framebuffer [][]cell
  layers.go            # composição: bg -> zona -> deco -> sprite -> balão
  sprite.go            # blit de sprite
  bubble.go            # desenho do balão com cauda
  hud.go               # header + footer (lista de agentes)
  compact.go           # fallback pra terminal pequeno
internal/tui/
  model.go             # bubbletea Model
  update.go            # Msg handling (tick, event, resize, key)
assets/
  assets.go            # embed.FS (embed não sobe diretório)
  sprites/
    charmander.txt
    squirtle.txt
    ...
```

---

## 4. Protocolo

### 4.1 Payload do Claude Code (stdin do hook)

```go
type HookEvent struct {
    SessionID      string          `json:"session_id"`
    TranscriptPath string          `json:"transcript_path"`
    CWD            string          `json:"cwd"`
    HookEventName  string          `json:"hook_event_name"`
    ToolName       string          `json:"tool_name,omitempty"`
    ToolInput      json.RawMessage `json:"tool_input,omitempty"`
    ToolResponse   json.RawMessage `json:"tool_response,omitempty"`
    Prompt         string          `json:"prompt,omitempty"` // UserPromptSubmit
}
```

> **Validar em runtime.** O schema dos hooks pode divergir por versão. O shim
> faz unmarshal tolerante (campos desconhecidos ignorados) e o daemon guarda o
> raw JSON pra debug (`--debug` dumpa em `/tmp/pokeclaude-events.jsonl`).

### 4.2 Envelope (shim → daemon)

```go
type Envelope struct {
    ReceivedAt time.Time       `json:"received_at"`
    PID        int             `json:"pid"`
    Event      HookEvent       `json:"event"`
    Raw        json.RawMessage `json:"raw"`
}
```

`POST /event` → `202 Accepted`, body vazio. Daemon nunca bloqueia: canal
bufferizado (`cap 256`), drop com log se cheio.

---

## 5. Shim

Requisito duro: rápido, nunca falha, nunca imprime nada no stdout.

1. `io.ReadAll(os.Stdin)` com limite de 1MB
2. unmarshal tolerante → HookEvent (erro? segue com Raw só)
3. monta Envelope
4. `http.Client{ Transport: unixTransport, Timeout: 50ms }`
5. POST; ignora qualquer erro
6. `os.Exit(0)`

Sem retry. Sem log em stderr — só se `POKECLAUDE_DEBUG=1`.

---

## 6. Modelo de domínio

```go
type AgentID string // = session_id (subagente: session_id + "#" + n)
type State int
const (
    StateIdle State = iota // wander aleatório
    StateThinking          // parado, balão "..." pulsando
    StateWorking           // bounce vertical, balão com a tool
    StateError             // shake horizontal, balão 💥
    StateDone              // sprite feliz, fade após 10s
)
```

### Determinismo do sprite

```go
func SpeciesFor(sessionID string) Species {
    h := fnv.New32a()
    h.Write([]byte(sessionID))
    return dex[h.Sum32()%uint32(len(dex))]
}
```

Mesma sessão → mesmo Pokémon, sempre. Subagente usa `hash(parentID + idx)`.

---

## 7. Reducer: evento → comportamento

| `hook_event_name` | Ação |
|---|---|
| `SessionStart` | garante zona do `cwd`; spawna agente; `StateIdle`; balão `"olá! 👋"` (3s) |
| `UserPromptSubmit` | `StateThinking`; balão = truncate(prompt) |
| `PreToolUse` | `StateWorking`; balão = `FromTool(toolName, toolInput)` |
| `PreToolUse` + `Task` | além do acima, spawna subagente na mesma zona |
| `PostToolUse` | se erro → `StateError` (2s) + shake, senão volta pra `StateThinking` |
| `Stop` | `StateDone`; balão `"✅ pronto"`; fade em 10s |
| `SubagentStop` | despawn do subagente com fade 2s |
| `SessionEnd` | despawn do agente **e de seus subagentes**; zona vazia liberada após 30s |

**Garbage collect**: no tick, agente com `LastSeen > 15min` é despawnado.

### Geração do balão (`FromTool`)

| Tool | Balão |
|---|---|
| `Read`/`Glob`/`Grep` | 🔍 lendo `<base>` |
| `Edit`/`Write` | ✏ editando `<base>` |
| `Bash` | ⚡ `<command>` |
| `Task` | 🥚 chamando ajuda |
| `WebFetch`/`WebSearch` | 🌐 `<host|query>` |
| `TodoWrite` | 📋 planejando |
| default | ⚙ `<toolName>` |

Sanitizar: strip newline, strip ANSI, truncate rune-safe, largura via
`runewidth` (emoji ocupa 2 colunas).

---

## 8. Zonas e layout

- Zona nasce sob demanda no primeiro evento de um `cwd` novo.
- Bin-packing simples: 1 zona ocupa tudo; 2 → colunas; 3–4 → grid 2×2;
  \>4 → grid 2×2 das mais ativas + paginação com `Tab`.
- Reflow em `WindowSizeMsg`: recalcula `Rect`, reposiciona agentes
  proporcionalmente (clamp dentro do novo rect).

### Decoração por `ZoneKind` (`hash(cwd) % 3`)

| Kind | Deco |
|---|---|
| `ZoneGrass` | tufos `"`, verde |
| `ZoneLake` | faixa `~~~~` na base, ciano, anima |
| `ZoneCave` | pedras `^`/`*`, cinza |

Deco semeada por `hash(cwd)` → estável entre reflows.

---

## 9. Movimento (tick = 50ms, 20fps)

- **Idle**: escolhe alvo aleatório na zona, lerp a 6 cél/s, dwell 2–5s, walk cycle.
- **Thinking**: parado, balão pulsa `.` → `..` → `...`.
- **Working**: bounce vertical (`|sin|`, 1 cél, 300ms).
- **Error**: shake horizontal (±1, 120ms).

Colisão entre agentes: repulsão suave quando distância < 6 colunas.

---

## 10. Render

Framebuffer `[]Cell` (rune + estilo). Composição em camadas, uma passada por
tick: zonas → deco → sprites (z-order por `Pos.Y`) → balões → HUD.

**Wide runes** (emoji): ao escrever emoji na coluna X, marca X+1 como célula
fantasma e pula no flush; runs de mesmo estilo são agrupados.

### Sprites (`assets/sprites/<species>.txt`)

```
# name: charmander
# color: 208
# frames: 3
--- idle
 (>_<)
 /|o|\
  d b
--- walk-a
...
--- walk-b
...
```

Parser lê no `init()` via `embed.FS`. Espelhamento horizontal para
`Facing == -1` (tabela de swap: `(`↔`)`, `/`↔`\`, `>`↔`<`, `d`↔`b`, `J`↔`L`).
Subagente: linha do meio + `°` em cima.

### Balão

Largura = `min(24, texto+2)`, quebra em até 2 linhas. Cauda alinhada com o
centro do sprite, clamped na borda. Se não cabe acima da zona, desenha abaixo
com cauda invertida.

### HUD e modo compacto

Header (nome, contadores, relógio, paginação) + footer (até 5 agentes por
`LastSeen`). Abaixo de **80×24**: modo compacto, só a lista, sem mapa.

---

## 11. Install

1. Lê `~/.claude/settings.json` (cria se não existir).
2. Backup em `settings.json.bak-<ts>`.
3. Merge **não-destrutivo**: se já existe entrada com `pokeclaude hook`, não
   duplica; preserva hooks de terceiros e outras chaves.
4. Usa caminho **absoluto** do binário (`os.Executable()`).

Eventos: `SessionStart`, `UserPromptSubmit`, `PreToolUse` (matcher `*`),
`PostToolUse` (matcher `*`), `Stop`, `SubagentStop`, `SessionEnd`.

---

## 12. Socket

Path: `$XDG_RUNTIME_DIR/pokeclaude.sock`, fallback `$TMPDIR/pokeclaude-$UID.sock`.

- Daemon: socket existe + conecta → outro daemon rodando, aborta. Recusou →
  socket stale, remove e segue.
- `chmod 0600`, `defer os.Remove`.

---

## 13. Fases

1. **Esqueleto jogável** — dex, world, render, tui, `demo` com eventos sintéticos.
2. **Pipeline real** — protocol, hook, ingest, install, doctor, reducer completo.
3. **Polimento** — balões inteligentes, deco animada, multi-zona + reflow, modo
   compacto, subagentes, estados error/done.
4. **Extras (backlog)** — mais espécies, cores por linguagem, level-up, record/replay.

---

## 14. Decisões fixas

- Sem persistência. Fechou o TUI, perdeu o estado. É um brinquedo.
- Sem config file na v1. Flags apenas.
- Sem TCP. Unix socket only (Windows fica fora da v1).
- Sprites originais em ASCII — nada de arte copiada.

---

## 15. Testes

- `world/reducer_test.go` — evento → estado esperado.
- `world/zone_test.go` — bin-packing determinístico, `FromTool`, sanitização.
- `render/render_test.go` — runas largas não desalinham, balões (largura,
  quebra, cauda).
- `render/smoke_test.go` — frame completo e compacto renderizam sem panic.
- `dex/dex_test.go` — `SpeciesFor` determinístico e distribuição uniforme.
- `hook/shim_test.go` — JSON malformado não panica.
- `ingest/integration_test.go` — transporte shim → daemon fim-a-fim.
- `world/space_test.go` — física do Space Drift (limites, warp, colisões, cap de
  obstáculos, auto-fire).
- `render/space_test.go` — render do Space Drift (dimensões, warp sem panic,
  orientação da nave).

---

## 17. Modos de movimentação (extensível)

O `World` tem um `Mode` (`ModeZone` | `ModeSpaceDrift`), alternável em runtime
com `m`. O reducer e a máquina de estados dos agentes são **compartilhados**
entre os modos; só a camada de movimento e de render muda.

- **Tick**: `switch mode` — zona usa `Agent.Step` + repulsão; Space Drift chama
  `SpaceField.step`.
- **Snapshot**: em Space Drift, popula `View.Space` (naves, obstáculos, tiros,
  explosões, estrelas, intensidade de warp).
- **Render**: `RenderFrame` ramifica em `drawMap` ou `drawSpace`.

### Space Drift

`internal/world/space.go` — sub-simulação:

- **Naves** (uma por agente): deriva com velocidade, wrap toroidal nas bordas,
  cruzeiro modulado pelo estado (impulso em Working, lento em Thinking).
- **Auto-desvio**: acelera perpendicular ao obstáculo mais próximo à frente.
- **Auto-fire**: dispara laser no obstáculo à frente dentro do alcance, com
  cooldown.
- **Obstáculos**: asteroides e cometas nascem das bordas, atravessam e saem;
  cap proporcional à área.
- **Colisões**: laser×obstáculo (dano + explosão) e nave×obstáculo (tremida +
  knockback).
- **Dobra espacial (warp)**: `triggerWarp` teleporta cada nave para outro
  quadrante no meio da animação; `warp` global alimenta o efeito de distorção.
- **Starfield**: rola pra esquerda (linhas de velocidade); acelera com impulso
  e explode em túnel durante o warp.

`internal/render/space.go` desenha tudo (naves orientadas por velocidade,
chama de propulsor, rastro de dobra `=`, explosões, cometas com cauda) com
clipping na área do mapa.

---

## 18. Modo Game — Arena (rinha estilo Mortal Kombat)

Modo **gráfico** (raylib), separado do TUI: subcomando `pokeclaude arena`. Segue
o mesmo princípio de separar **engine** de **render** — o motor de combate é
Go puro, determinístico e testável headless; o raylib fica atrás da build tag
`raylib`.

### Engine (`internal/arena/`, puro)

- **Fighter** com stats derivados do dex (determinísticos por espécie):
  `Ataque`, `Defesa`, `Esquiva/Velocidade`, `Combo`, **Vida (HP)**, **Mana**,
  **Vidas/Rounds** (melhor de 2N-1).
- **Máquina de estados:** Idle, Andar, Ataque (startup→active→recovery),
  Bloqueio, Esquiva (i-frames), Hitstun, K.O.
- **Golpes com frame data** (`move.go`): leve, pesado, especial (gasta mana),
  finalização (super, gasta barra). Reach, knockback, hitstun, chip no bloqueio.
- **Resolução de hit:** hitbox ativa vs. distância; esquiva com i-frames anula;
  bloqueio reduz a dano de chip; combos encadeiam enquanto o alvo está em
  hitstun (com *scaling* de dano influenciado pelo stat Combo).
- **Fluxo:** intro → luta (timer) → round over → próximo round / fim de partida;
  vitória impecável quando o vencedor não toma dano.
- Passo fixo (`Advance` acumula em `fixedDT`), determinístico; `View()` gera o
  snapshot de render; `Log()` guarda o histórico para o modo texto.

### Controle (`controller.go`, `control.go`)

- Interface `Controller` polada a **~250ms** (`decisionInterval`) — não a 60fps,
  para caber a latência de um LLM.
- `BotController` — heurístico determinístico (oponente/teste).
- `RemoteController` — alimentado de fora; ações one-shot (ataques/esquiva/super)
  disparam uma vez, ações "seguradas" (andar/bloquear) persistem.
- `ControlServer` — socket unix (`pokeclaude-arena.sock`) que roteia comandos
  `{side, action}` para o `RemoteController` do lado. CLI: `pokeclaude arena-cmd`.
  Assim um **agente de IA** gerencia o galo injetando comandos (IA × IA).

### Render (`internal/arena/rl/`, `//go:build raylib`)

Apresentação estilo MK, **procedural e original**: lutadores desenhados com
formas (pose por estado: ataque, bloqueio, esquiva com after-images, hitstun,
K.O.), barras de HP/mana/super, pips de round, timer, hit-sparks, screen shake,
e anúncios ("ROUND 1 — LUTAR!", "K.O.", "VITÓRIA IMPECÁVEL").

### Empacotamento

- `pokeclaude arena` roda **headless** (texto) por padrão neste/qualquer
  ambiente; com `-tags raylib` (+ libs OpenGL/X11/Wayland) abre a janela.
- Engine e testes compilam sem raylib. **Não rodar `go mod tidy`** sem
  `-tags raylib`, senão a dependência do raylib some do `go.mod`.
