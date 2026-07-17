package world

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/mpgxc/pokeclaude/internal/dex"
	"github.com/mpgxc/pokeclaude/internal/protocol"
)

// Apply mutates the world in response to a single hook envelope.
func (w *World) Apply(e protocol.Envelope) {
	w.mu.Lock()
	defer w.mu.Unlock()

	ev := e.Event
	now := e.ReceivedAt
	if now.IsZero() {
		now = time.Now()
	}
	if ev.SessionID == "" {
		return
	}
	id := AgentID(ev.SessionID)

	switch ev.HookEventName {
	case protocol.HookSessionStart:
		a := w.spawnRoot(id, ev.CWD, now)
		a.setState(StateIdle)
		a.Thought = Thought{Icon: '👋', Text: "olá!", Expires: now.Add(greetDuration)}

	case protocol.HookUserPromptSubmit:
		a := w.touch(id, ev.CWD, now)
		a.setState(StateThinking)
		a.Thought = Thought{Icon: '💭', Text: sanitize(ev.Prompt)}

	case protocol.HookPreToolUse:
		a := w.touch(id, ev.CWD, now)
		a.setState(StateWorking)
		a.LastTool = ev.ToolName
		a.Thought = FromTool(ev.ToolName, ev.ToolInput)
		if ev.ToolName == "Task" {
			w.spawnSub(a, now)
		}

	case protocol.HookPostToolUse:
		a := w.touch(id, ev.CWD, now)
		if isError(ev.ToolResponse) {
			a.setState(StateError)
			a.Thought = Thought{Icon: '💥', Text: "erro", Expires: now.Add(errorDuration)}
		} else {
			a.setState(StateThinking)
		}

	case protocol.HookStop:
		a := w.touch(id, ev.CWD, now)
		a.setState(StateDone)
		a.Thought = Thought{Icon: '✅', Text: "pronto"}
		a.fadeAt = now.Add(doneFade)

	case protocol.HookSubagentStop:
		w.stopNewestSub(id, now)

	case protocol.HookSessionEnd:
		w.endSession(id, now)
	}

	if z := w.zones[slug(ev.CWD)]; z != nil {
		z.LastActive = now
	}
}

// spawnRoot ensures the zone and a root agent for the session, resetting fades.
func (w *World) spawnRoot(id AgentID, cwd string, now time.Time) *Agent {
	z := w.ensureZone(cwd, now)
	a := w.agents[id]
	if a == nil {
		a = w.newAgent(id, "", string(id), z, now)
		w.agents[id] = a
		w.order = append(w.order, id)
	}
	a.ZoneID = z.ID
	a.LastSeen = now
	a.fadeAt = time.Time{}
	a.removing = false
	return a
}

// touch returns the agent for id, lazily spawning it (and its zone) if the
// daemon missed the SessionStart. Keeps LastSeen fresh.
func (w *World) touch(id AgentID, cwd string, now time.Time) *Agent {
	a := w.agents[id]
	if a == nil {
		return w.spawnRoot(id, cwd, now)
	}
	a.LastSeen = now
	a.fadeAt = time.Time{}
	return a
}

// ensureZone returns the zone for cwd, creating and laying it out if new.
func (w *World) ensureZone(cwd string, now time.Time) *Zone {
	sid := slug(cwd)
	if z, ok := w.zones[sid]; ok {
		z.LastActive = now
		z.emptyAt = time.Time{}
		return z
	}
	z := newZone(cwd, w.nextZoneIdx, now)
	w.nextZoneIdx++
	w.zones[z.ID] = z
	w.relayout()
	return z
}

// newAgent builds a fresh agent placed at a random point in its zone.
func (w *World) newAgent(id, parent AgentID, sessionKey string, z *Zone, now time.Time) *Agent {
	sp := dex.SpeciesFor(sessionKey)
	a := &Agent{
		ID:        id,
		ParentID:  parent,
		Species:   sp,
		Nick:      sp.Name,
		ZoneID:    z.ID,
		Facing:    1,
		State:     StateIdle,
		LastSeen:  now,
		SpawnedAt: now,
	}
	if z.Rect.W > 0 {
		a.Pos = z.randomPoint(w.rng, sp.Idle.W, sp.Idle.H)
	} else {
		a.Pos = Vec2{X: float32(z.Rect.X + 1), Y: float32(z.Rect.Y + 1)}
	}
	a.Target = a.Pos
	a.baseX, a.baseY = a.Pos.X, a.Pos.Y
	return a
}

// spawnSub creates a subagent under parent a in the same zone.
func (w *World) spawnSub(parent *Agent, now time.Time) {
	parent.subCount++
	key := string(parent.ID) + "#sub" // vary species by parent + index
	subKey := key + itoa(parent.subCount)
	id := AgentID(string(parent.ID) + "#" + itoa(parent.subCount))
	z := w.zones[parent.ZoneID]
	if z == nil {
		return
	}
	sub := w.newAgent(id, parent.ID, subKey, z, now)
	sub.setState(StateWorking)
	sub.Thought = Thought{Icon: '🥚', Text: "ajudando"}
	w.agents[id] = sub
	w.order = append(w.order, id)
}

// stopNewestSub fades out the most recently spawned subagent of a session.
func (w *World) stopNewestSub(parent AgentID, now time.Time) {
	var newest *Agent
	for _, a := range w.agents {
		if a.ParentID == parent {
			if newest == nil || a.SpawnedAt.After(newest.SpawnedAt) {
				newest = a
			}
		}
	}
	if newest != nil {
		newest.setState(StateDone)
		newest.fadeAt = now.Add(subagentFade)
	}
}

// endSession fades out an agent and all its subagents.
func (w *World) endSession(id AgentID, now time.Time) {
	if a := w.agents[id]; a != nil {
		a.setState(StateDone)
		a.fadeAt = now.Add(subagentFade)
	}
	for _, c := range w.agents {
		if c.ParentID == id {
			c.fadeAt = now.Add(subagentFade)
		}
	}
}

// isError inspects a tool response for an error signal, tolerating any shape.
func isError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err == nil {
		if v, ok := m["is_error"].(bool); ok && v {
			return true
		}
		if v, ok := m["error"]; ok && v != nil {
			return true
		}
		if s, ok := m["status"].(string); ok && strings.EqualFold(s, "error") {
			return true
		}
		return false
	}
	// non-object response: look for an error marker in the text
	return strings.Contains(strings.ToLower(string(raw)), "error")
}

// itoa is a tiny non-allocating-ish integer formatter (avoids strconv import
// churn and keeps ids compact).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// SortedZoneIDs returns zone ids sorted, for stable test assertions.
func (w *World) SortedZoneIDs() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	ids := make([]string, 0, len(w.zones))
	for id := range w.zones {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// AgentState returns the state of an agent by id, for tests. ok is false if the
// agent is absent.
func (w *World) AgentState(id AgentID) (State, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if a, ok := w.agents[id]; ok {
		return a.State, true
	}
	return 0, false
}
