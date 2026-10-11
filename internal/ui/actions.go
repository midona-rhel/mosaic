package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type action struct {
	Label, Shortcut, Hint string
	HoverOwner            string
	Key                   tea.KeyPressMsg
	Danger, Informational bool
	Rect                  Rect
}

func keyAction(label string, code rune, text string) action {
	k := tea.KeyPressMsg{Code: code, Text: text}
	shortcut := k.String()
	switch code {
	case tea.KeyEnter:
		shortcut = "enter"
	case tea.KeyEscape:
		shortcut = "esc"
	case tea.KeyLeft:
		shortcut = "←"
	case tea.KeyRight:
		shortcut = "→"
	case ' ':
		shortcut = "space"
	}
	return action{Label: label, Shortcut: shortcut, Key: k}
}

// Keys stay visible at the bottom left, with brighter keys and quieter verbs,
// following Charm Bubbles' short-help pattern. Grouped navigation hints have
// no click target; individual command shortcuts remain clickable.
func (m *Model) actions() []action {
	close := keyAction("close", tea.KeyEscape, "")
	as := m.contextActions(close)
	// Preserve discoverability when only two or three shortcuts fit.
	if m.width < 44 {
		for i, a := range as {
			if a.Key.Code == '?' && i > 1 {
				copy(as[2:i+1], as[1:i])
				as[1] = a
				break
			}
		}
	}
	return as
}

func (m *Model) contextActions(close action) []action {
	if m.drag != nil && m.drag.Moved && m.drag.Kind != "scroll" {
		close.Label = "cancel"
		return []action{close, {Shortcut: "release", Label: "place", Informational: true}}
	}
	if m.organizing {
		toggle := "remove"
		if m.disabled[m.order[m.organizerIndex]] {
			toggle = "add"
		}
		return []action{keyAction(toggle, ' ', " "), keyAction("first", tea.KeyEnter, ""), keyAction("sort", 's', "s"), keyAction("open", 'f', "f"), close}
	}
	help := keyAction("help", '?', "?")
	navigation := func(shortcut, label string) action {
		return action{Shortcut: shortcut, Label: label, Informational: true}
	}
	if m.picking {
		return []action{keyAction("choose", tea.KeyEnter, ""), close, navigation("↑↓", "select")}
	}
	if m.confirm {
		close.Label = "cancel"
		terminate := keyAction("terminate", 'y', "y")
		terminate.Danger = true
		return []action{close, terminate}
	}
	if m.help {
		return []action{close, navigation("↑↓", "scroll")}
	}
	if m.target != nil {
		as := []action{close, navigation("↑↓", "scroll")}
		if !m.opts.ReadOnly && m.inspection != nil {
			a := keyAction("terminate", 'x', "x")
			a.Danger = true
			as = append(as, a)
		}
		return append(as, help)
	}
	if m.editing {
		clear := keyAction("clear", 'u', "")
		clear.Key.Mod = tea.ModCtrl
		clear.Shortcut = "ctrl+u"
		return []action{keyAction("done", tea.KeyEnter, ""), clear}
	}
	if m.reordering {
		close.Label = "cancel"
		return []action{keyAction("done", tea.KeyEnter, ""), close, navigation("↑↓←→", "move"), navigation("shift+arrows", "size"), keyAction("sort", 's', "s"), keyAction("reset", '0', "0")}
	}
	var as []action
	if m.expanded || m.activeWindow() >= 0 {
		close.Label = "back"
		if m.activeWindow() >= 0 {
			close.Label = "dock"
		}
		as = append(as, close)
		label := "scroll"
		if m.kind(m.selected) == 4 {
			label = "select"
		}
		as = append(as, navigation("↑↓", label))
		if m.activeWindow() >= 0 && len(m.windows) > 1 {
			as = append(as, keyAction("window", tea.KeyTab, ""))
		}
		if m.kind(m.selected) == 4 {
			as = append(as, keyAction("inspect", tea.KeyEnter, ""), keyAction("search", '/', "/"), keyAction("sort", 's', "s"))
		} else if m.activeWindow() >= 0 {
			as = append(as, keyAction("maximize", 'm', "m"))
		} else {
			as = append(as, keyAction("pop-out", 'f', "f"))
		}
	} else {
		as = []action{keyAction("open", tea.KeyEnter, ""), navigation("↑↓←→", "move"), keyAction("float", 'f', "f")}
		if m.width >= 100 {
			as = append(as, keyAction("layout", 'p', "p"))
		}
		as = append(as, keyAction("panels", 'o', "o"))
	}
	if !m.expanded && m.activeWindow() < 0 && m.kind(m.selected) == 4 {
		as = append(as, keyAction(strings.ToLower(m.sortBy), 's', "s"))
	}
	if !m.expanded && m.activeWindow() < 0 {
		as = append(as, keyAction("hide", 'H', "H"))
	}
	if m.expanded || m.activeWindow() >= 0 {
		as = append(as, keyAction("replace", 't', "t"))
	}
	as = append(as, help)
	if m.expanded || m.activeWindow() >= 0 {
		as = append(as, keyAction("refresh", 'r', "r"))
	}
	if !m.expanded && m.activeWindow() < 0 {
		label := "pause"
		if m.paused {
			label = "resume"
		}
		as = append(as, keyAction(label, ' ', " "), keyAction("quit", 'q', "q"))
	}
	return as
}

func (m *Model) actionBar() ([]action, string) {
	x := 1
	gap := 2
	if m.confirm {
		gap = 1
	}
	as := []action{}
	var line strings.Builder
	shown := 0
	line.WriteByte(' ')
	for _, a := range m.actions() {
		w := lipgloss.Width(a.Shortcut) + 1 + lipgloss.Width(a.Label)
		if x+w > m.width-1 {
			break
		}
		a.Rect = Rect{x, m.height - 1, w, 1}
		if !a.Informational {
			as = append(as, a)
		}
		fg := cyan
		bg := background
		if a.Danger {
			fg = red
		}
		if !a.Informational && a.Rect.Contains(m.mouseX, m.mouseY) {
			bg = lipgloss.Color("#332845")
		}
		if shown > 0 {
			line.WriteString(strings.Repeat(" ", gap))
		}
		line.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(true).Render(a.Shortcut))
		line.WriteString(lipgloss.NewStyle().Foreground(muted).Background(bg).Render(" " + a.Label))
		shown++
		x += w + gap
	}
	return as, line.String()
}
