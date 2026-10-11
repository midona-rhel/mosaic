package ui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *Model) click(v tea.MouseClickMsg) tea.Cmd {
	if m.opts.NoMouse || v.Button != tea.MouseLeft {
		return nil
	}
	m.mouseX, m.mouseY = v.X, v.Y
	if m.organizing {
		return m.organizerClick(v)
	}
	if m.picking {
		return m.pickerClick(v)
	}
	as, _ := m.actionBar()
	for _, a := range as {
		if a.Rect.Contains(v.X, v.Y) {
			return m.key(a.Key)
		}
	}
	if m.clickScrollbar(v) {
		return nil
	}
	if m.clickProcessSort(v) {
		return nil
	}
	if m.help {
		m.help = false
		return nil
	}
	if m.confirm {
		for _, a := range m.confirmationActions() {
			if a.Rect.Contains(v.X, v.Y) {
				return m.key(a.Key)
			}
		}
		return nil
	}
	if m.target != nil {
		r := m.inspectorRect()
		for _, a := range m.inspectorActions() {
			if a.Rect.Contains(v.X, v.Y) {
				return m.key(a.Key)
			}
		}
		if e := edgesAt(r, v.X, v.Y); e != 0 {
			m.drag = &gesture{Kind: "inspector-resize", StartX: v.X, StartY: v.Y, Origin: r, Edges: e}
			return nil
		}
		if r.Contains(v.X, v.Y) && v.Y == r.Y+1 {
			m.drag = &gesture{Kind: "inspector", StartX: v.X, StartY: v.Y, Origin: r, GrabX: v.X - r.X, GrabY: v.Y - r.Y}
			return nil
		}
		if !r.Contains(v.X, v.Y) {
			m.target = nil
			m.inspection = nil
		}
		return nil
	}
	if !m.expanded {
		if handled, cmd := m.windowClick(v); handled {
			return cmd
		}
	}
	if m.expanded {
		for _, a := range m.expandedActions() {
			if a.Rect.Contains(v.X, v.Y) {
				return m.key(a.Key)
			}
		}
		firstRow := 5 + m.processHeaderRows()
		if m.kind(m.selected) == 4 && v.Y >= firstRow && v.Y < m.height-3 {
			i := m.offset + v.Y - firstRow
			if i < len(m.processes()) {
				m.chooseProcess(i)
				return m.openProcess()
			}
		}
		return nil
	}
	l := m.layout()
	for slot, r := range l.Cards {
		i := m.order[slot]
		if v.Y >= 2 && v.Y < m.height-2 && r.Contains(v.X, v.Y) {
			m.focusWindow = -1
			m.selected = i
			if m.cardHide(i).Rect.Contains(v.X, v.Y) {
				m.reordering = false
				m.drag = nil
				m.hideSelectedPanel()
				return nil
			}
			if m.cardPopout(i).Rect.Contains(v.X, v.Y) {
				m.reordering = false
				m.drag = nil
				m.floatPanel(i)
				return nil
			}
			if e := edgesAt(r, v.X, v.Y); e != 0 {
				m.drag = &gesture{Kind: "card-resize", Panel: i, StartX: v.X, StartY: v.Y, Origin: r, Edges: e, SpanOrigin: m.span(i)}
				return nil
			}
			if v.Y <= r.Y+1 {
				m.drag = &gesture{Kind: "card", Panel: i, StartX: v.X, StartY: v.Y, GrabX: v.X - r.X, GrabY: v.Y - r.Y, Origin: r}
				return nil
			}
			return nil
		}
	}
	return nil
}
