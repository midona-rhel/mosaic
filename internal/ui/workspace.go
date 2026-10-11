package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"sort"
)

func (m *Model) slot(panel int) int {
	for i, p := range m.order {
		if p == panel {
			return i
		}
	}
	return 0
}

// Insert the dragged card at its destination; other cards shift in order.
// Stable panel IDs keep hotkeys, telemetry and open windows attached to a card.
func (m *Model) moveCard(panel, target int) {
	from := m.slot(panel)
	if from == target {
		return
	}
	if from < target {
		copy(m.order[from:target], m.order[from+1:target+1])
	} else {
		copy(m.order[target+1:from+1], m.order[target:from])
	}
	m.order[target] = panel
}

func (m *Model) cardRect(panel int) Rect {
	r := m.layout().Cards[m.slot(panel)]
	return r
}

func cardOwner(panel int) string   { return fmt.Sprintf("card-%d", panel) }
func windowOwner(panel int) string { return fmt.Sprintf("window-%d", panel) }

func (m *Model) cardPopout(panel int) action {
	r := m.cardRect(panel)
	a := keyAction("↗", 'f', "f")
	a.HoverOwner = fmt.Sprintf("card-%d", panel)
	a.Hint = "Pop out " + m.panelTitle(panel) + " · f"
	if m.windowIndex(panel) >= 0 {
		a.Hint = "Raise " + m.panelTitle(panel) + " window"
	}
	a.Rect = Rect{r.X + r.W - 5, r.Y + 1, 3, 1}
	return a
}

func (m *Model) hideAction(panel int, r Rect, owner string, offset int) action {
	a := keyAction("-", 'H', "H")
	a.HoverOwner = owner
	a.Hint = "Hide " + m.panelTitle(panel) + " · H · restore with o"
	a.Rect = Rect{r.X + r.W - offset, r.Y + 1, 3, 1}
	return a
}

func (m *Model) cardHide(panel int) action {
	return m.hideAction(panel, m.cardRect(panel), fmt.Sprintf("card-%d", panel), 9)
}

// Visibility changes never change a panel's saved priority, content or span.
func (m *Model) setPanelEnabled(panel int, enabled bool) {
	m.disabled[panel] = !enabled
	if !enabled {
		if i := m.windowIndex(panel); i >= 0 {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
		}
		if m.focusWindow == panel {
			m.focusWindow = -1
		}
		if m.selected == panel {
			m.expanded = false
			m.offset = 0
		}
	}
	m.reveal()
}

func (m *Model) hideSelectedPanel() {
	if m.expanded || m.activeWindow() >= 0 {
		return
	}
	p := m.selected
	m.setPanelEnabled(p, false)
	m.status = "Hidden " + m.panelTitle(p) + " · o to restore"
}

func (m *Model) windowActions(w floatingWindow) []action {
	r := w.Bounds
	up := keyAction("↑", 'm', "m")
	up.HoverOwner = fmt.Sprintf("window-%d", w.Panel)
	up.Hint = "Maximize " + m.panelTitle(w.Panel) + " · m"
	up.Rect = Rect{r.X + r.W - 9, r.Y + 1, 3, 1}
	dock := keyAction("x", tea.KeyEscape, "")
	dock.HoverOwner = up.HoverOwner
	dock.Hint = "Dock " + m.panelTitle(w.Panel) + " · Esc"
	dock.Rect = Rect{r.X + r.W - 5, r.Y + 1, 3, 1}
	return []action{up, dock}
}

func (m *Model) inspectorActions() []action {
	r := m.inspectorRect()
	close := keyAction("x", tea.KeyEscape, "")
	close.HoverOwner = "inspector"
	close.Hint = "Close inspector · Esc"
	close.Rect = Rect{r.X + r.W - 5, r.Y + 1, 3, 1}
	as := []action{close}
	if !m.opts.ReadOnly && m.inspection != nil {
		a := keyAction("Terminate", 'x', "x")
		a.HoverOwner = "inspector"
		a.Hint = "Request termination · x"
		a.Danger = true
		a.Rect = Rect{r.X + 2, r.Y + r.H - 2, 11, 1}
		as = append(as, a)
	}
	return as
}

func (m *Model) expandedActions() []action {
	r := m.expandedRect()
	pop := keyAction("↗", 'f', "f")
	pop.HoverOwner = "expanded"
	pop.Hint = "Pop out " + m.panelTitle(m.selected) + " · f"
	pop.Rect = Rect{r.X + r.W - 9, r.Y + 1, 3, 1}
	close := keyAction("x", tea.KeyEscape, "")
	close.HoverOwner = "expanded"
	close.Hint = "Back to dashboard · Esc"
	close.Rect = Rect{r.X + r.W - 5, r.Y + 1, 3, 1}
	return []action{pop, close}
}

func (m *Model) confirmationRect() Rect {
	w, h := min(62, m.width-4), min(12, m.height-2)
	return Rect{(m.width - w) / 2, (m.height - h) / 2, w, h}
}

func (m *Model) confirmationActions() []action {
	r := m.confirmationRect()
	cancel := keyAction("Cancel", tea.KeyEscape, "")
	cancel.HoverOwner = "confirmation"
	cancel.Rect = Rect{r.X + 2, r.Y + r.H - 2, 8, 1}
	a := keyAction("Terminate", 'y', "y")
	a.HoverOwner = "confirmation"
	a.Danger = true
	a.Rect = Rect{r.X + 11, r.Y + r.H - 2, 11, 1}
	if r.W-4 < 20 {
		cancel.Rect.Y--
		a.Rect.X = r.X + 2
	}
	return []action{cancel, a}
}

func (m *Model) control(a action) string {
	c, bg := ink, lipgloss.Color("#332845")
	if a.Danger {
		c = red
		bg = lipgloss.Color("#3C2031")
	}
	if m.drag == nil && !m.opts.NoMouse && a.Rect.Contains(m.mouseX, m.mouseY) && a.HoverOwner != "" && a.HoverOwner == m.hoverOwnerAt(m.mouseX, m.mouseY) {
		c = ink
		bg = lipgloss.Color("#503575")
		if a.Danger {
			bg = lipgloss.Color("#832B48")
		} else {
			c = cyan
		}
	}
	return lipgloss.NewStyle().Foreground(c).Background(bg).Bold(true).Render(" " + a.Label + " ")
}

func (m *Model) hoverHint() string {
	if m.confirm || m.help || m.picking || m.organizing {
		return ""
	}
	if m.target != nil {
		for _, a := range m.inspectorActions() {
			if a.Rect.Contains(m.mouseX, m.mouseY) {
				return a.Hint
			}
		}
		return ""
	}
	if m.expanded {
		for _, a := range m.expandedActions() {
			if a.Rect.Contains(m.mouseX, m.mouseY) {
				return a.Hint
			}
		}
		return ""
	}
	if m.mouseY < 2 || m.mouseY >= m.height-2 {
		return ""
	}
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if !w.Bounds.Contains(m.mouseX, m.mouseY) {
			continue
		}
		for _, a := range m.windowActions(w) {
			if a.Rect.Contains(m.mouseX, m.mouseY) {
				return a.Hint
			}
		}
		if edgesAt(w.Bounds, m.mouseX, m.mouseY) != 0 {
			return "Drag edge to resize · Esc cancels"
		}
		return ""
	}
	if m.drag != nil && m.drag.Kind == "card-resize" {
		return "Resize in grid steps · Esc cancels"
	}
	for _, panel := range m.order {
		if edgesAt(m.cardRect(panel), m.mouseX, m.mouseY) != 0 {
			return "Drag edge to resize in grid steps · Shift+arrows"
		}
		for _, a := range []action{m.cardPopout(panel), m.cardHide(panel)} {
			if a.Rect.Contains(m.mouseX, m.mouseY) {
				return a.Hint
			}
		}
	}
	if m.drag != nil && m.drag.Kind == "card" && m.drag.Moved {
		return "Drop on a card to rearrange · Esc cancels"
	}
	return ""
}

// Repeated sort toggles between alphabetical titles and the original order.
// Size preferences belong to panel identities, not their sorted positions.
func (m *Model) sortCards() {
	want := defaultOrder()
	sort.SliceStable(want[:], func(i, j int) bool {
		if m.disabled[want[i]] != m.disabled[want[j]] {
			return !m.disabled[want[i]]
		}
		return m.panelTitle(want[i]) < m.panelTitle(want[j])
	})
	if m.order == want {
		want = defaultOrder()
	}
	m.order = want
	m.reveal()
}
