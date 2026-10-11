package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

type scrollArea struct {
	Owner                  string
	Panel                  int
	Rect                   Rect
	Total, Visible, Offset int
}

func (a scrollArea) thumb() (int, int) {
	height := max(1, a.Rect.H*a.Visible/max(1, a.Total))
	travel := a.Rect.H - height
	start := travel * min(max(0, a.Offset), max(0, a.Total-a.Visible)) / max(1, a.Total-a.Visible)
	return a.Rect.Y + start, height
}

func (m *Model) windowRect(w floatingWindow) Rect {
	if g := m.drag; g != nil && g.Moved && g.Valid && g.Panel == w.Panel && (g.Kind == "move" || g.Kind == "resize") {
		return g.Preview
	}
	return w.Bounds
}

func (m *Model) scrollAreas() []scrollArea {
	if m.organizing || m.picking || m.confirm {
		return nil
	}
	var areas []scrollArea
	add := func(a scrollArea) {
		if a.Visible > 0 && a.Rect.H > 0 && a.Total > a.Visible {
			areas = append(areas, a)
		}
	}
	if m.help {
		r := m.modalRect()
		add(scrollArea{"help", -1, Rect{r.X + r.W - 2, r.Y + 1, 1, r.H - 2}, len(m.helpLines()), r.H - 2, m.helpOffset})
		return areas
	}
	if m.target != nil {
		r := m.inspectorRect()
		add(scrollArea{"inspector", -1, Rect{r.X + r.W - 2, r.Y + 3, 1, r.H - 6}, len(m.processDetailLines(r.W - 4)), r.H - 6, m.modalOffset})
		return areas
	}
	panel := func(p int, r Rect, offset int, owner string) {
		vm := *m
		vm.selected, vm.expanded, vm.focusWindow = p, true, -1
		vm.windows = nil
		vm.height, vm.width = r.H+4, r.W+2
		total, visible, y := 0, r.H-4, r.Y+3
		if m.kind(p) == 4 {
			header := vm.processHeaderRows()
			total = len(vm.processes())
			visible -= header
			y += header
		} else {
			total = len(vm.detailLines(r.W - 4))
		}
		add(scrollArea{owner, p, Rect{r.X + r.W - 2, y, 1, visible}, total, visible, offset})
	}
	if m.expanded {
		panel(m.selected, m.expandedRect(), m.offset, "expanded")
		return areas
	}
	for _, p := range m.order {
		if m.kind(p) != 3 {
			continue
		}
		r := m.cardRect(p)
		if r.W == 0 {
			continue
		}
		count := diskCardCount(r.H - 2)
		add(scrollArea{cardOwner(p), p, Rect{r.X + r.W - 2, r.Y + 2, 1, max(1, r.H-3)}, len(visibleVolumes(m.snapshot.Volumes)), count, m.diskStart(p, count)})
	}
	for _, w := range m.windows {
		offset := w.Offset
		if w.Panel == m.focusWindow {
			offset = m.offset
		}
		panel(w.Panel, m.windowRect(w), offset, windowOwner(w.Panel))
	}
	return areas
}

func (m *Model) scrollAreaVisible(a scrollArea, x, y int) bool {
	if a.Owner == "help" {
		return m.help
	}
	return m.hoverOwnerAt(x, y) == a.Owner
}

func (m *Model) paintScrollbars(canvas *lipgloss.Canvas) {
	for _, a := range m.scrollAreas() {
		start, height := a.thumb()
		for y := a.Rect.Y; y < a.Rect.Y+a.Rect.H; y++ {
			if !m.scrollAreaVisible(a, a.Rect.X, y) {
				continue
			}
			ch, color := "│", lipgloss.Color("#36334E")
			if y >= start && y < start+height {
				ch, color = "┃", cyan
			}
			canvas.SetCell(a.Rect.X, y, &uv.Cell{Content: ch, Width: 1, Style: uv.Style{Fg: color, Bg: background}})
		}
	}
}

func (m *Model) scrollbarAt(x, y int) (scrollArea, bool) {
	for _, a := range m.scrollAreas() {
		if a.Rect.Contains(x, y) && m.scrollAreaVisible(a, x, y) {
			return a, true
		}
	}
	return scrollArea{}, false
}

func (m *Model) setScrollOffset(a scrollArea, offset int) {
	offset = min(max(0, offset), max(0, a.Total-a.Visible))
	switch a.Owner {
	case "help":
		m.helpOffset = offset
	case "inspector":
		m.modalOffset = offset
	default:
		if a.Owner == cardOwner(a.Panel) {
			m.diskOffset[a.Panel] = offset
			return
		}
		if i := m.windowIndex(a.Panel); i >= 0 {
			m.raise(i)
		}
		m.selected = a.Panel
		if m.kind(a.Panel) == 4 {
			m.chooseProcess(min(max(m.processIndex(), offset), offset+a.Visible-1))
		}
		m.offset = offset
		m.clampOffsets()
	}
}

func (m *Model) clickScrollbar(v tea.MouseClickMsg) bool {
	a, ok := m.scrollbarAt(v.X, v.Y)
	if !ok {
		return false
	}
	start, height := a.thumb()
	grab := v.Y - start
	if v.Y < start || v.Y >= start+height {
		grab = height / 2
		m.setScrollOffset(a, (v.Y-a.Rect.Y-grab)*max(0, a.Total-a.Visible)/max(1, a.Rect.H-height))
	}
	m.drag = &gesture{Kind: "scroll", Panel: a.Panel, StartX: v.X, StartY: v.Y, GrabY: grab, Scroll: a}
	return true
}
