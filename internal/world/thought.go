package world

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// maxThoughtWidth is the display-column budget for bubble text.
const maxThoughtWidth = 24

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// sanitize strips control characters and ANSI escapes, collapses whitespace and
// truncates to maxThoughtWidth display columns (rune- and wide-rune-safe).
func sanitize(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	return truncateWidth(s, maxThoughtWidth)
}

// truncateWidth truncates s to at most w display columns, appending "…" if cut.
func truncateWidth(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if used+rw > w-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	b.WriteRune('…')
	return b.String()
}

// FromTool builds the bubble content for a PreToolUse event, based on the tool
// name and its (raw JSON) input.
func FromTool(toolName string, toolInput json.RawMessage) Thought {
	in := parseToolInput(toolInput)
	switch toolName {
	case "Read", "Glob", "Grep":
		target := firstNonEmpty(in["file_path"], in["pattern"], in["path"])
		return thought('🔍', "lendo "+base(target))
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return thought('✏', "editando "+base(in["file_path"]))
	case "Bash":
		return thought('⚡', in["command"])
	case "Task":
		return thought('🥚', "chamando ajuda")
	case "WebFetch":
		return thought('🌐', host(in["url"]))
	case "WebSearch":
		return thought('🌐', in["query"])
	case "TodoWrite":
		return thought('📋', "planejando")
	default:
		return thought('⚙', toolName)
	}
}

// thought assembles a Thought, sanitizing the text.
func thought(icon rune, text string) Thought {
	return Thought{Icon: icon, Text: sanitize(text)}
}

// parseToolInput does a tolerant unmarshal of the tool input into a flat
// string map. Non-string values are ignored.
func parseToolInput(raw json.RawMessage) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// base returns the final path element, or the input unchanged if it has none.
func base(p string) string {
	if p == "" {
		return "?"
	}
	return filepath.Base(p)
}

// host extracts the host from a URL, falling back to the raw string.
func host(raw string) string {
	if raw == "" {
		return "?"
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}
