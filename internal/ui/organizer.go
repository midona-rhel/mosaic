package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) organizerRect() Rect {
	w, h := min(68, m.width-2), min(panelCount+5, m.height-4)
	return Rect{(m.width - w) / 2, (m.height - h) / 2, w, h}
}
func (m *Model) organizerPage() (int, int) {
	count := max(1, m.organizerRect().H-5)
	return m.organizerIndex / count * count, count
}
func (m *Model) organizerLines(width int) []string {
	start, count := m.organizerPage()
	title := "PANELS / PRIORITY ORDER"
	if count < panelCount {
		title = fmt.Sprintf("PANELS · %d/%d", start/count+1, (panelCount+count-1)/count)
	}
	lines := []string{strong.Render(title), dim.Render("space add/remove · shift+↑↓ move · s sort"), ""}
	l := m.layout()
	for slot := start; slot < min(panelCount, start+count); slot++ {
		p := m.order[slot]
		r := l.Cards[slot]
		s := m.span(p)
		state := "Hidden"
		if m.disabled[p] {
			state = "Removed"
		} else if r.W > 0 {
			state = fmt.Sprintf("r%d c%d", (r.Y-2)/l.RowStep+1, (r.X-1)/l.ColumnStep+1)
		}
		name := fmt.Sprintf(" %d %s", p+1, m.panelTitle(p))
		if slot == m.organizerIndex {
			name = "› " + name[1:]
		}
		line := pair(name, fmt.Sprintf("%d×%d %s", s.W, s.H, state), width)
		if slot == m.organizerIndex {
			line = strong.Foreground(accent).Render(line)
		} else {
			line = dim.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}
func (m *Model) organizerKey(k tea.KeyPressMsg) tea.Cmd {
	switch key := k.String(); key {
	case "esc", "o":
		m.organizing = false
	case "up", "k":
		m.organizerIndex = (m.organizerIndex + panelCount - 1) % panelCount
	case "down", "j", "tab":
		m.organizerIndex = (m.organizerIndex + 1) % panelCount
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.organizerIndex = m.slot(int(key[0] - '1'))
	case "space", "+", "-", "delete":
		p := m.order[m.organizerIndex]
		removed := !m.disabled[p]
		if key == "+" {
			removed = false
		}
		if key == "-" || key == "delete" {
			removed = true
		}
		m.setPanelEnabled(p, !removed)
		if !removed {
			m.selected = p
		}
		m.reveal()
	case "shift+up", "shift+down":
		slot := m.organizerIndex
		next := slot - 1
		if key == "shift+down" {
			next = slot + 1
		}
		if next >= 0 && next < panelCount {
			m.moveCard(m.order[slot], next)
			m.organizerIndex = next
		}
	case "s":
		p := m.order[m.organizerIndex]
		m.sortCards()
		m.organizerIndex = m.slot(p)
	case "enter":
		p := m.order[m.organizerIndex]
		m.disabled[p] = false
		m.moveCard(p, 0)
		m.selected, m.focusWindow, m.expanded, m.organizing = p, -1, false, false
		m.reveal()
	case "f":
		m.selected = m.order[m.organizerIndex]
		m.organizing, m.expanded = false, true
		m.reordering = false
		m.focusWindow, m.offset = -1, 0
		m.ensureProcess()
	}
	return nil
}
func (m *Model) organizerRowAt(x, y int) (int, bool) {
	r := m.organizerRect()
	start, count := m.organizerPage()
	i := y - (r.Y + 4)
	return start + i, r.Contains(x, y) && x > r.X && x < r.X+r.W-1 && i >= 0 && i < count && start+i < panelCount
}
func (m *Model) organizerClick(v tea.MouseClickMsg) tea.Cmd {
	as, _ := m.actionBar()
	for _, a := range as {
		if a.Rect.Contains(v.X, v.Y) {
			return m.organizerKey(a.Key)
		}
	}
	if i, ok := m.organizerRowAt(v.X, v.Y); ok {
		m.organizerIndex = i
		return nil
	}
	if !m.organizerRect().Contains(v.X, v.Y) {
		m.organizing = false
	}
	return nil
}
