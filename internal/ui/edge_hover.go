package ui

import "fmt"

// A soft whole-border hover identifies clickable panels without borrowing the
// selected panel's purple gradient. Resize edges add a stronger cyan cue.
func (m *Model) hoveredSurface() Rect {
	if m.opts.NoMouse || m.drag != nil || m.help || m.confirm || m.picking || m.organizing {
		return Rect{}
	}
	owner := m.hoverOwnerAt(m.mouseX, m.mouseY)
	if m.target != nil {
		return Rect{}
	}
	for _, w := range m.windows {
		if owner == fmt.Sprintf("window-%d", w.Panel) && w.Panel != m.focusWindow {
			return w.Bounds
		}
	}
	if m.expanded {
		return Rect{}
	}
	for slot, r := range m.layout().Cards {
		if owner == fmt.Sprintf("card-%d", m.order[slot]) && (m.order[slot] != m.selected || m.activeWindow() >= 0) {
			return r
		}
	}
	return Rect{}
}

// Hover belongs to the topmost surface, including its blank content. A
// covered button must not light up even if part of that button remains visible.
func (m *Model) hoverOwnerAt(x, y int) string {
	if m.confirm {
		if m.confirmationRect().Contains(x, y) {
			return "confirmation"
		}
		return ""
	}
	if m.help || m.picking || m.organizing {
		return ""
	}
	if m.target != nil {
		if m.inspectorRect().Contains(x, y) {
			return "inspector"
		}
		return ""
	}
	if y < 2 || y >= m.height-2 {
		return ""
	}
	if m.expanded {
		if (m.expandedRect()).Contains(x, y) {
			return "expanded"
		}
		return ""
	}
	for i := len(m.windows) - 1; i >= 0; i-- {
		if m.windowRect(m.windows[i]).Contains(x, y) {
			return fmt.Sprintf("window-%d", m.windows[i].Panel)
		}
	}
	for slot, r := range m.layout().Cards {
		if r.Contains(x, y) {
			return fmt.Sprintf("card-%d", m.order[slot])
		}
	}
	return ""
}

// Only the topmost actionable border is highlighted. Keyboard selection keeps
// its purple gradient; hovering an edge paints just that edge bright cyan.
func (m *Model) hoveredEdge() (Rect, resizeEdge) {
	if m.opts.NoMouse || m.help || m.confirm || m.picking || m.organizing || m.drag != nil {
		return Rect{}, 0
	}
	x, y := m.mouseX, m.mouseY
	if m.target != nil {
		r := m.inspectorRect()
		return r, edgesAt(r, x, y)
	}
	if m.expanded || y < 2 || y >= m.height-2 {
		return Rect{}, 0
	}
	for i := len(m.windows) - 1; i >= 0; i-- {
		r := m.windows[i].Bounds
		if r.Contains(x, y) {
			return r, edgesAt(r, x, y)
		}
	}
	for _, r := range m.layout().Cards {
		if r.Contains(x, y) {
			return r, edgesAt(r, x, y)
		}
	}
	return Rect{}, 0
}

func onHighlightedEdge(r Rect, e resizeEdge, x, y int) bool {
	return e != 0 && r.Contains(x, y) && (e&resizeLeft != 0 && x == r.X || e&resizeRight != 0 && x == r.X+r.W-1 || e&resizeTop != 0 && y == r.Y || e&resizeBottom != 0 && y == r.Y+r.H-1)
}
