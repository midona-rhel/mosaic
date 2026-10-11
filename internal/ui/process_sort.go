package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

type processColumn struct {
	Key, Label string
	Start, End int
}

func processColumns(w int, overview bool) []processColumn {
	header := pad("PID", 9) + pad("NAME", max(0, w-30)) + " " + pad("CPU %", 7) + "  " + pad("MEMORY", 10)
	if w < 36 {
		header = "  PID CPU %  NAME"
	}
	if overview {
		header = ansi.Strip(overviewProcessHeader(w))
	}
	var cols []processColumn
	for x := 0; x < len(header) && x < w; x++ {
		for _, v := range []struct{ label, key string }{{"PID", "PID"}, {"NAME", "Name"}, {"CPU %", "CPU"}, {"MEMORY", "Memory"}, {"MEM", "Memory"}} {
			if strings.HasPrefix(header[x:], v.label) {
				if len(cols) > 0 {
					cols[len(cols)-1].End = x
				}
				cols = append(cols, processColumn{v.key, v.label, x, w})
				x += len(v.label) - 1
				break
			}
		}
	}
	return cols
}

func (m *Model) processHeader(w int, overview bool) string {
	result := ""
	cursor := 0
	for _, c := range processColumns(w, overview) {
		result += strings.Repeat(" ", max(0, c.Start-cursor))
		label := c.Label
		style := dim
		if c.Key == m.sortBy {
			arrow := "↓"
			if (c.Key == "Name" || c.Key == "PID") != m.sortReverse {
				arrow = "↑"
			}
			if c.End-c.Start > len(label) {
				label += arrow
			}
			style = strong.Foreground(cyan)
		}
		result += style.Render(pad(label, c.End-c.Start))
		cursor = c.End
	}
	return result
}

func (m *Model) processSortAt(x, y int) (int, string, bool) {
	if m.help || m.confirm || m.picking || m.organizing || m.target != nil || m.drag != nil {
		return 0, "", false
	}
	owner := m.hoverOwnerAt(x, y)
	check := func(p, ox, oy, w int, overview bool) (int, string, bool) {
		if m.kind(p) != 4 || y != oy {
			return 0, "", false
		}
		for _, c := range processColumns(w, overview) {
			if x >= ox+c.Start && x < ox+c.End {
				return p, c.Key, true
			}
		}
		return 0, "", false
	}
	if m.expanded {
		if m.processHeaderRows() != 3 {
			return 0, "", false
		}
		return check(m.selected, 3, 7, m.width-6, false)
	}
	for _, w := range m.windows {
		if w.Bounds.Contains(x, y) && owner == windowOwner(w.Panel) && w.Bounds.H-4 > 3 {
			return check(w.Panel, w.Bounds.X+2, w.Bounds.Y+5, w.Bounds.W-4, false)
		}
	}
	for _, p := range m.order {
		r := m.cardRect(p)
		if !r.Contains(x, y) || owner != cardOwner(p) {
			continue
		}
		row := r.Y + 3
		if r.H-2 < 6 {
			row = r.Y + 2
		}
		return check(p, r.X+2, row, r.W-4, true)
	}
	return 0, "", false
}

func (m *Model) clickProcessSort(v tea.MouseClickMsg) bool {
	p, key, ok := m.processSortAt(v.X, v.Y)
	if !ok {
		return false
	}
	if i := m.windowIndex(p); i >= 0 && m.windows[i].Bounds.Contains(v.X, v.Y) {
		m.raise(i)
	} else {
		m.focusWindow = -1
	}
	m.selected = p
	if key == m.sortBy {
		m.sortReverse = !m.sortReverse
	} else {
		m.sortBy = key
		m.sortReverse = false
	}
	m.offset = 0
	m.chooseProcess(0)
	return true
}
