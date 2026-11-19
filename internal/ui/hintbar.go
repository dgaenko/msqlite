package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// hintView is a footer bar whose cells always span the full available width.
type hintView struct {
	*tview.TextView
	pairs []string
}

func newHintView() *hintView {
	v := &hintView{TextView: tview.NewTextView().SetDynamicColors(true)}
	v.SetBackgroundColor(colHintBg)
	v.SetTextColor(colHintFg)
	return v
}

// SetHints stores key/label pairs (key1, label1, key2, label2, ...).
func (v *hintView) SetHints(pairs ...string) {
	v.pairs = pairs
	v.refresh()
}

func (v *hintView) refresh() {
	width := 80
	if _, _, w, _ := v.GetRect(); w > 0 {
		width = w
	}
	v.TextView.SetText(hintBarWidth(width, v.pairs...))
}

func (v *hintView) Draw(screen tcell.Screen) {
	v.refresh()
	v.TextView.Draw(screen)
}

// hintBarWidth lays key/label pairs out in equal-width cells filling width.
func hintBarWidth(width int, pairs ...string) string {
	n := len(pairs) / 2
	if width <= 0 || n == 0 {
		return ""
	}
	cell := width / n
	extra := width % n
	var b strings.Builder
	for i := 0; i < n; i++ {
		w := cell
		if i < extra {
			w++
		}
		b.WriteString(hintCell(pairs[i*2], pairs[i*2+1], w))
	}
	return b.String()
}

func hintCell(key, label string, width int) string {
	if width <= 0 {
		return ""
	}
	key = tr(key)
	label = tr(label)

	maxKey := width - 2
	if maxKey < 1 {
		maxKey = width
	}
	if tview.TaggedStringWidth(key) > maxKey {
		key = fitHintText(key, maxKey)
	}
	keyText := fmt.Sprintf("[%s:%s]%s[-:-:-]", colorName(colKeyFg), colorName(colKeyBg), key)
	keyWidth := tview.TaggedStringWidth(keyText)

	content := keyText
	if room := width - keyWidth - 1; room > 0 {
		content += " " + fitHintText(label, room)
	}

	used := tview.TaggedStringWidth(content)
	if used >= width {
		return content
	}
	pad := width - used
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + content + strings.Repeat(" ", right)
}

func fitHintText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}
