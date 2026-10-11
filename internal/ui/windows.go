package ui

import (
	"fmt"
	"math"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type floatingWindow struct {
	Panel  int
	Bounds Rect
	Offset int
}
type gesture struct {
	Kind                                string
	Panel, StartX, StartY, GrabX, GrabY int
	Origin                              Rect
	Moved                               bool
	Edges                               resizeEdge
	SpanOrigin                          Span
	Preview                             Rect
	PreviewSpan                         Span
	PreviewOrder                        [panelCount]int
	Valid                               bool
	Scroll                              scrollArea
}

type resizeEdge uint8

const (
	resizeLeft resizeEdge = 1 << iota
	resizeRight
	resizeTop
	resizeBottom
)

// Only the visible border is a resize handle; title buttons and content stay
// independent. Corners combine two edges.
func edgesAt(r Rect, x, y int) resizeEdge {
	if !r.Contains(x, y) {
		return 0
	}
	var e resizeEdge
	if x == r.X {
		e |= resizeLeft
	}
	if x == r.X+r.W-1 {
		e |= resizeRight
	}
	if y == r.Y {
		e |= resizeTop
	}
	if y == r.Y+r.H-1 {
		e |= resizeBottom
	}
	return e
}

// Keep the opposite edge anchored, including at minimum size and viewport
// boundaries. Clamping a resized rectangle afterwards would move that anchor.
func resizeBounds(r Rect, e resizeEdge, dx, dy, width, height int) Rect {
	left, right, top, bottom := r.X, r.X+r.W, r.Y, r.Y+r.H
	minW, minH := min(24, max(4, width-2)), min(8, max(4, height-4))
	if e&resizeLeft != 0 {
		left = max(1, min(left+dx, right-minW))
	}
	if e&resizeRight != 0 {
		right = min(width-1, max(right+dx, left+minW))
	}
	if e&resizeTop != 0 {
		top = max(2, min(top+dy, bottom-minH))
	}
	if e&resizeBottom != 0 {
		bottom = min(height-2, max(bottom+dy, top+minH))
	}
	return Rect{left, top, right - left, bottom - top}
}

func (m *Model) activeWindow() int {
	if m.expanded {
		return -1
	}
	for i, w := range m.windows {
		if w.Panel == m.focusWindow {
			return i
		}
	}
	return -1
}
func (m *Model) windowIndex(panel int) int {
	for i, w := range m.windows {
		if w.Panel == panel {
			return i
		}
	}
	return -1
}
func clampWindow(r Rect, width, height int) Rect {
	maxW, maxH := max(4, width-2), max(4, height-4)
	r.W = min(maxW, max(min(24, maxW), r.W))
	r.H = min(maxH, max(min(8, maxH), r.H))
	r.X = min(max(1, r.X), max(1, width-r.W-1))
	r.Y = min(max(2, r.Y), max(2, height-r.H-2))
	return r
}
func (m *Model) floatPanel(panel int) {
	if m.width < 24 || m.height < 10 {
		return
	}
	if i := m.activeWindow(); i >= 0 {
		m.windows[i].Offset = m.offset
	}
	m.expanded = false
	m.selected = panel
	i := m.windowIndex(panel)
	if i < 0 {
		w, h := min(62, m.width-4), min(22, m.height-6)
		if m.kind(panel) == 0 {
			r := m.expandedRect()
			w, h = r.W, r.H
		}
		r := clampWindow(Rect{(m.width-w)/2 + len(m.windows)*2, (m.height-h)/2 + len(m.windows), w, h}, m.width, m.height)
		m.windows = append(m.windows, floatingWindow{panel, r, m.offset})
		i = len(m.windows) - 1
	}
	m.raise(i)
	m.status = "Floating " + m.panelTitle(panel)
}
func (m *Model) raise(index int) {
	m.reordering = false
	if old := m.activeWindow(); old >= 0 {
		m.windows[old].Offset = m.offset
	}
	w := m.windows[index]
	m.windows = append(append(m.windows[:index:index], m.windows[index+1:]...), w)
	m.focusWindow = w.Panel
	m.selected = w.Panel
	m.offset = w.Offset
	m.clampOffsets()
}
func (m *Model) dock(panel int) {
	i := m.windowIndex(panel)
	if i < 0 {
		return
	}
	m.windows = append(m.windows[:i], m.windows[i+1:]...)
	m.focusWindow = -1
	m.offset = 0
	if len(m.windows) > 0 {
		m.raise(len(m.windows) - 1)
	} else {
		m.selected = panel
		m.reveal()
	}
	m.status = "Docked " + m.panelTitle(panel)
}
func (m *Model) maximizeWindow() {
	i := m.activeWindow()
	if i < 0 {
		return
	}
	w := m.windows[i]
	m.windows = append(m.windows[:i], m.windows[i+1:]...)
	m.focusWindow = -1
	m.selected = w.Panel
	m.expanded = true
	m.offset = w.Offset
	m.drag = nil
	m.clampOffsets()
	m.status = "Maximized " + m.panelTitle(w.Panel)
}
func (m *Model) inspectorRect() Rect {
	if m.draggingInspector() && m.drag.Valid {
		return m.drag.Preview
	}
	if m.inspectorBounds != nil {
		return clampWindow(*m.inspectorBounds, m.width, m.height)
	}
	return m.modalRect()
}
func (m *Model) motion(v tea.MouseMotionMsg) {
	if m.opts.NoMouse {
		return
	}
	m.mouseX, m.mouseY = v.X, v.Y
	if m.opts.NoMouse || m.drag == nil || (m.help && m.drag.Kind != "scroll") || m.confirm || m.picking {
		return
	}
	g := m.drag
	if v.Button != tea.MouseLeft {
		return
	}
	dx, dy := v.X-g.StartX, v.Y-g.StartY
	if dx == 0 && dy == 0 && !g.Moved {
		return
	}
	g.Moved = true
	if g.Kind == "scroll" {
		_, height := g.Scroll.thumb()
		offset := (v.Y - g.Scroll.Rect.Y - g.GrabY) * max(0, g.Scroll.Total-g.Scroll.Visible) / max(1, g.Scroll.Rect.H-height)
		m.setScrollOffset(g.Scroll, offset)
		g.Preview.Y = offset
		return
	}
	if g.Kind == "card" {
		g.Valid = false
		if v.Y < 2 || v.Y >= m.height-2 {
			return
		}
		for _, w := range m.windows {
			if w.Bounds.Contains(v.X, v.Y) {
				return
			}
		}
		for slot, r := range m.layout().Cards {
			if r.Contains(v.X, v.Y) {
				vm := *m
				vm.moveCard(g.Panel, slot)
				g.Preview, g.PreviewOrder = vm.cardRect(g.Panel), vm.order
				g.Valid = g.Preview.W > 0
				return
			}
		}
		return
	}
	if g.Kind == "card-resize" {
		base, limits := m.layout(), m.panelSpanLimits(g.Panel)
		s := g.SpanOrigin
		if g.Edges&(resizeLeft|resizeRight) != 0 {
			if g.Edges&resizeLeft != 0 {
				dx = -dx
			}
			if steps := int(math.Round(float64(dx) / float64(base.ColumnStep))); steps != 0 {
				s.W = min(limits.W, max(1, min(s.W, limits.W)+steps))
			}
		}
		if g.Edges&(resizeTop|resizeBottom) != 0 {
			if g.Edges&resizeTop != 0 {
				dy = -dy
			}
			if steps := int(math.Round(float64(dy) / float64(base.RowStep))); steps != 0 {
				s.H = min(limits.H, max(1, min(s.H, limits.H)+steps))
			}
		}
		vm := *m
		// Growing up/left inserts before the card occupying the new origin.
		x, y := g.Origin.X, g.Origin.Y
		if g.Edges&resizeTop != 0 {
			y -= (min(s.H, limits.H) - min(g.SpanOrigin.H, limits.H)) * base.RowStep
		}
		if g.Edges&resizeLeft != 0 {
			x -= (min(s.W, limits.W) - min(g.SpanOrigin.W, limits.W)) * base.ColumnStep
		}
		for slot, r := range base.Cards {
			if r.Contains(max(1, x), max(2, y)) && slot < vm.slot(g.Panel) {
				vm.moveCard(g.Panel, slot)
				break
			}
		}
		vm.setCardSpan(g.Panel, s)
		g.Preview, g.PreviewSpan, g.PreviewOrder = vm.cardRect(g.Panel), s, vm.order
		g.Valid = g.Preview.W > 0
		return
	}
	r := g.Origin
	if g.Kind == "resize" || g.Kind == "inspector-resize" {
		r = resizeBounds(r, g.Edges, dx, dy, m.width, m.height)
	} else {
		r.X, r.Y = v.X-g.GrabX, v.Y-g.GrabY
		r = clampWindow(r, m.width, m.height)
	}
	g.Preview, g.Valid = r, true
}
func (m *Model) release(v tea.MouseReleaseMsg) {
	if m.drag == nil {
		return
	}
	g := m.drag
	if g.Moved && v.Button == tea.MouseLeft {
		m.motion(tea.MouseMotionMsg{X: v.X, Y: v.Y, Button: tea.MouseLeft})
	}
	m.drag = nil
	if g.Kind == "scroll" {
		return
	}
	if g.Moved && g.Valid {
		switch g.Kind {
		case "card", "card-resize":
			m.order = g.PreviewOrder
			if g.Kind == "card-resize" {
				m.spans[g.Panel] = g.PreviewSpan
			}
			m.selected = g.Panel
			m.reveal()
		case "inspector", "inspector-resize":
			r := g.Preview
			m.inspectorBounds = &r
			m.clampOffsets()
		default:
			if i := m.windowIndex(g.Panel); i >= 0 {
				m.windows[i].Bounds = g.Preview
				m.clampOffsets()
			}
		}
		return
	}
}

func (m *Model) windowClick(v tea.MouseClickMsg) (bool, tea.Cmd) {
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		r := w.Bounds
		if !r.Contains(v.X, v.Y) {
			continue
		}
		m.raise(i)
		if e := edgesAt(r, v.X, v.Y); e != 0 {
			m.drag = &gesture{Kind: "resize", Panel: w.Panel, StartX: v.X, StartY: v.Y, Origin: r, Edges: e}
			return true, nil
		}
		for _, a := range m.windowActions(w) {
			if a.Rect.Contains(v.X, v.Y) {
				return true, m.key(a.Key)
			}
		}
		if v.Y == r.Y+1 {
			m.drag = &gesture{Kind: "move", Panel: w.Panel, StartX: v.X, StartY: v.Y, GrabX: v.X - r.X, GrabY: v.Y - r.Y, Origin: r}
			return true, nil
		}
		if m.kind(w.Panel) == 4 {
			first := r.Y + 3 + m.processHeaderRows()
			if v.Y >= first && v.Y < r.Y+r.H-1 {
				index := m.offset + v.Y - first
				if index < len(m.processes()) {
					m.chooseProcess(index)
					return true, m.openProcess()
				}
			}
		}
		return true, nil
	}
	return false, nil
}
func (m *Model) windowLines(w floatingWindow) []string {
	r := w.Bounds
	vm := *m
	vm.selected = w.Panel
	vm.expanded = true
	vm.height = r.H + 4
	vm.width = r.W + 2
	vm.offset = w.Offset
	if m.focusWindow == w.Panel {
		vm.offset = m.offset
	}
	as := m.windowActions(w)
	title := pair(strong.Render(":: "+m.panelTitle(w.Panel)), m.control(as[0])+" "+m.control(as[1]), r.W-4)
	lines := []string{title, ""}
	bodyH := max(1, r.H-4)
	if m.kind(w.Panel) == 4 {
		lines = append(lines, vm.processTable(r.W-4, bodyH)...)
	} else {
		all := vm.detailLines(r.W - 4)
		start := min(vm.offset, len(all))
		lines = append(lines, all[start:min(len(all), start+bodyH)]...)
	}
	for len(lines) < r.H-2 {
		lines = append(lines, "")
	}
	return lines
}
func (m *Model) floatingLayers() []*lipgloss.Layer {
	layers := []*lipgloss.Layer{}
	for i, w := range m.windows {
		if m.drag != nil && m.drag.Moved && m.drag.Valid && m.drag.Panel == w.Panel && (m.drag.Kind == "move" || m.drag.Kind == "resize") {
			w.Bounds = m.drag.Preview
		}

		r := w.Bounds
		color := "#4F4770"
		if w.Panel == m.focusWindow {
			color = "#B45CFF"
		}
		z := 3 + i*2
		content := m.dragContent(fmt.Sprintf("window-%d", w.Panel), r.W, r.H, func() string { return box(m.windowLines(w), r.W, r.H, color) })
		layers = append(layers, lipgloss.NewLayer(content).X(r.X).Y(r.Y).Z(z+1).ID(fmt.Sprintf("window-%d", w.Panel)))
	}
	return layers
}
