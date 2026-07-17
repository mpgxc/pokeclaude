package render

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/mpgxc/pokeclaude/internal/world"
)

// maxBubbleInner is the widest text area (display columns) inside a bubble.
const maxBubbleInner = 22

// bubbleStyle is the muted color used for the bubble border and text.
var bubbleStyle = Style{Fg: "252"}

// layoutBubble wraps the icon+text into at most two lines, each padded to the
// common inner width. It returns the lines and that inner width.
func layoutBubble(icon rune, text string) ([]string, int) {
	content := text
	if icon != 0 {
		content = string(icon) + " " + text
	}
	lines := wrapWidth(content, maxBubbleInner, 2)
	inner := 0
	for _, l := range lines {
		if w := runewidth.StringWidth(l); w > inner {
			inner = w
		}
	}
	if inner == 0 {
		inner = 1
	}
	padded := make([]string, len(lines))
	for i, l := range lines {
		padded[i] = padRightWidth(l, inner)
	}
	return padded, inner
}

// BubbleArt renders the full bubble as plain text lines, including the tail.
// tailCol is the column within the inner area (0..inner+1) where the tail
// attaches. When below is true the tail points up (bubble sits under the
// sprite); otherwise it points down.
func BubbleArt(icon rune, text string, tailCol int, below bool) []string {
	body, inner := layoutBubble(icon, text)
	span := inner + 2 // dashes across the top/bottom border
	tc := tailCol
	if tc < 0 {
		tc = 0
	}
	if tc > span-1 {
		tc = span - 1
	}

	plainTop := "╭" + strings.Repeat("─", span) + "╮"
	notched := notchBorder(span, tc, below)
	content := make([]string, len(body))
	for i, l := range body {
		content[i] = "│ " + l + " │"
	}
	tailLine := strings.Repeat(" ", tc+1) + "│"

	var out []string
	if below {
		out = append(out, tailLine)
		out = append(out, notched) // top border carries the notch
		out = append(out, content...)
		out = append(out, "╰"+strings.Repeat("─", span)+"╯")
	} else {
		out = append(out, plainTop)
		out = append(out, content...)
		out = append(out, notched) // bottom border carries the notch
		out = append(out, tailLine)
	}
	return out
}

// notchBorder builds a horizontal border with a tail notch at column tc. For a
// bottom border (below=false) the corners are ╰ ╯ and the notch is ╮; for a top
// border (below=true) the corners are ╭ ╮ and the notch is ╯.
func notchBorder(span, tc int, below bool) string {
	left, right, notch := '╰', '╯', '╮'
	if below {
		left, right, notch = '╭', '╮', '╯'
	}
	var b strings.Builder
	b.WriteRune(left)
	for k := 0; k < span; k++ {
		if k == tc {
			b.WriteRune(notch)
		} else {
			b.WriteRune('─')
		}
	}
	b.WriteRune(right)
	return b.String()
}

// blitBubble draws an agent's thought bubble above (or below) its sprite.
func blitBubble(f *Frame, av world.AgentView, zone world.ZoneView) {
	if !av.Thought.Active(av.LastSeen) && av.Thought.Text == "" {
		// still show pulsing dots when thinking with no text
		if av.State != world.StateThinking {
			return
		}
	}
	icon, text := bubbleContent(av)
	if text == "" && icon == 0 {
		return
	}

	spriteW, _ := spriteFootprint(av)
	x0 := int(av.Pos.X + 0.5)
	y0 := int(av.Pos.Y + 0.5)
	spriteCenter := x0 + spriteW/2

	_, inner := layoutBubble(icon, text)
	boxW := inner + 4 // borders + padding
	// place bubble so its tail roughly aligns under the sprite center
	bx := spriteCenter - boxW/2
	if bx < zone.Rect.X+1 {
		bx = zone.Rect.X + 1
	}
	if bx+boxW > zone.Rect.Right()-1 {
		bx = zone.Rect.Right() - 1 - boxW
	}
	tailCol := spriteCenter - (bx + 1)

	below := y0-4 < zone.Rect.Y // not enough room above
	art := BubbleArt(icon, text, tailCol, below)

	var by int
	if below {
		by = y0 + spriteHeight(av)
	} else {
		by = y0 - len(art)
	}
	for dy, line := range art {
		f.SetString(bx, by+dy, line, bubbleStyle)
	}
}

// bubbleContent resolves the icon and text to show for an agent.
func bubbleContent(av world.AgentView) (rune, string) {
	if av.State == world.StateThinking && av.Thought.Text == "" {
		// pulsing dots based on the animation clock
		n := int(av.Clock/0.4)%3 + 1
		return '💭', strings.Repeat(".", n)
	}
	return av.Thought.Icon, av.Thought.Text
}

func spriteFootprint(av world.AgentView) (int, int) {
	if av.IsSub {
		return av.Species.Idle.W, 2
	}
	return av.Species.Idle.W, av.Species.Idle.H
}

func spriteHeight(av world.AgentView) int {
	if av.IsSub {
		return 2
	}
	return av.Species.Idle.H
}

// wrapWidth greedily wraps s into at most maxLines lines of at most width
// display columns. Overflow is truncated with an ellipsis on the last line.
func wrapWidth(s string, width, maxLines int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, w := range words {
		cand := w
		if cur != "" {
			cand = cur + " " + w
		}
		if runewidth.StringWidth(cand) <= width {
			cur = cand
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur = w
		if len(lines) == maxLines-1 {
			break
		}
	}
	if len(lines) < maxLines {
		lines = append(lines, cur)
	}
	// truncate any single word/line wider than width
	for i, l := range lines {
		if runewidth.StringWidth(l) > width {
			lines[i] = truncWidth(l, width)
		}
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

func truncWidth(s string, w int) string {
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

func padRightWidth(s string, w int) string {
	d := w - runewidth.StringWidth(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}
