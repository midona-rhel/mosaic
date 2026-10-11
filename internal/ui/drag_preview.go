package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

func (m *Model) draggingInspector() bool {
	return m.drag != nil && m.drag.Moved && (m.drag.Kind == "inspector" || m.drag.Kind == "inspector-resize")
}

func (m *Model) paintDragPreview(canvas *lipgloss.Canvas) {
	g := m.drag
	if g == nil || !g.Moved || !g.Valid || (g.Kind != "card" && g.Kind != "card-resize") {
		return
	}
	r := g.Preview
	border := lipgloss.RoundedBorder()
	put := func(x, y int, ch string) {
		if x < 0 || x >= m.width || y < 2 || y >= m.height-2 {
			return
		}
		canvas.SetCell(x, y, &uv.Cell{Content: ch, Width: 1, Style: uv.Style{Fg: ink, Bg: background}})
	}
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		put(x, r.Y, border.Top)
		put(x, r.Y+r.H-1, border.Bottom)
	}
	for y := r.Y + 1; y < r.Y+r.H-1; y++ {
		put(r.X, y, border.Left)
		put(r.X+r.W-1, y, border.Right)
	}
	put(r.X, r.Y, border.TopLeft)
	put(r.X+r.W-1, r.Y, border.TopRight)
	put(r.X, r.Y+r.H-1, border.BottomLeft)
	put(r.X+r.W-1, r.Y+r.H-1, border.BottomRight)
}

func (m *Model) previewHidden() []string {
	vm := *m
	if g := m.drag; g != nil && g.Moved && g.Valid && (g.Kind == "card" || g.Kind == "card-resize") {
		vm.order = g.PreviewOrder
		if g.Kind == "card-resize" {
			vm.spans[g.Panel] = g.PreviewSpan
		}
	}
	var names []string
	for slot, r := range vm.layout().Cards {
		if r.W == 0 && !vm.disabled[vm.order[slot]] {
			p := vm.order[slot]
			names = append(names, fmt.Sprintf("%d %s", p+1, m.panelTitle(p)))
		}
	}
	return names
}

func (m *Model) dragHint() string {
	g := m.drag
	if g == nil || !g.Moved {
		return ""
	}
	if !g.Valid {
		return "Move onto a grid panel · Esc cancel"
	}
	label := m.panelTitle(g.Panel)
	if m.draggingInspector() {
		label = "Process"
	}
	if g.Kind == "card-resize" {
		label += fmt.Sprintf(" %d×%d", g.PreviewSpan.W, g.PreviewSpan.H)
	}
	s := "Release to place · Esc cancel"
	if g.Kind == "card" || g.Kind == "card-resize" {
		if hidden := m.previewHidden(); len(hidden) > 0 {
			s = "Will hide: " + strings.Join(hidden, ", ") + " · release / Esc"
		}
	}
	return label + " · " + s
}
