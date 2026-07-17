package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hookCommand is a single command hook in Claude Code's settings.
type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookEntry groups hooks under an optional matcher.
type hookEntry struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

// hookEventSpec lists every hook event PokéClaude wires up and whether it needs
// a tool matcher.
var hookEventSpecs = []struct {
	Name    string
	Matcher string
}{
	{"SessionStart", ""},
	{"UserPromptSubmit", ""},
	{"PreToolUse", "*"},
	{"PostToolUse", "*"},
	{"Stop", ""},
	{"SubagentStop", ""},
	{"SessionEnd", ""},
}

// hookMarker identifies our command inside a settings file.
const hookMarker = "pokeclaude hook"

// settingsPath returns ~/.claude/settings.json.
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// readSettings loads the settings file into a generic top-level map, preserving
// every key. A missing file yields an empty map.
func readSettings(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("settings.json inválido: %w", err)
	}
	return m, nil
}

// writeSettings writes the settings map back, pretty-printed, creating parent
// directories as needed.
func writeSettings(path string, m map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

// backupSettings copies the current file to settings.json.bak-<ts>. It is a
// no-op if the file does not exist.
func backupSettings(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	bak := fmt.Sprintf("%s.bak-%d", path, time.Now().Unix())
	if err := os.WriteFile(bak, data, 0o644); err != nil {
		return "", err
	}
	return bak, nil
}

// decodeHooks pulls the "hooks" section out of the settings map.
func decodeHooks(m map[string]json.RawMessage) (map[string][]hookEntry, error) {
	hooks := map[string][]hookEntry{}
	raw, ok := m["hooks"]
	if !ok || len(raw) == 0 {
		return hooks, nil
	}
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return nil, fmt.Errorf("seção hooks inválida: %w", err)
	}
	return hooks, nil
}

// encodeHooks stores the hooks section back into the settings map (or removes
// it if empty).
func encodeHooks(m map[string]json.RawMessage, hooks map[string][]hookEntry) error {
	if len(hooks) == 0 {
		delete(m, "hooks")
		return nil
	}
	raw, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	m["hooks"] = raw
	return nil
}

// entryHasMarker reports whether an entry contains our hook command.
func entryHasMarker(e hookEntry) bool {
	for _, h := range e.Hooks {
		if strings.Contains(h.Command, hookMarker) {
			return true
		}
	}
	return false
}

// installHooks merges PokéClaude's hooks into the hooks map, non-destructively.
// It returns the number of events newly wired.
func installHooks(hooks map[string][]hookEntry, command string) int {
	added := 0
	for _, spec := range hookEventSpecs {
		entries := hooks[spec.Name]
		already := false
		for _, e := range entries {
			if entryHasMarker(e) {
				already = true
				break
			}
		}
		if already {
			continue
		}
		hooks[spec.Name] = append(entries, hookEntry{
			Matcher: spec.Matcher,
			Hooks:   []hookCommand{{Type: "command", Command: command}},
		})
		added++
	}
	return added
}

// uninstallHooks removes PokéClaude's hook commands, preserving third-party
// hooks. It returns the number of events touched.
func uninstallHooks(hooks map[string][]hookEntry) int {
	removed := 0
	for _, spec := range hookEventSpecs {
		entries, ok := hooks[spec.Name]
		if !ok {
			continue
		}
		var kept []hookEntry
		for _, e := range entries {
			var keptHooks []hookCommand
			for _, h := range e.Hooks {
				if !strings.Contains(h.Command, hookMarker) {
					keptHooks = append(keptHooks, h)
				} else {
					removed++
				}
			}
			if len(keptHooks) > 0 {
				e.Hooks = keptHooks
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(hooks, spec.Name)
		} else {
			hooks[spec.Name] = kept
		}
	}
	return removed
}
