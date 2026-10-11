package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) key(k tea.KeyPressMsg) tea.Cmd {
	key := k.String()
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.drag != nil && key == "esc" {
		m.drag = nil
		return nil
	}
	if m.drag != nil {
		return nil
	}
	if m.organizing {
		return m.organizerKey(k)
	}

	if m.picking {
		return m.pickerKey(k)
	}
	if m.confirm {
		if m.terminating {
			return nil
		}
		switch key {
		case "esc", "n":
			m.confirm = false
		case "y":
			return m.terminate()
		}
		return nil
	}
	if m.help {
		if key == "up" || key == "k" || key == "pgup" {
			m.helpOffset--
		}
		if key == "down" || key == "j" || key == "pgdown" {
			m.helpOffset++
		}
		m.helpOffset = min(max(0, m.helpOffset), max(0, len(m.helpLines())-(m.modalRect().H-2)))
		if key == "esc" || key == "?" {
			m.help = false
		}
		if key == "q" {
			return tea.Quit
		}
		return nil
	}
	if m.editing {
		switch key {
		case "esc", "enter":
			m.editing = false
		case "backspace":
			r := []rune(m.query)
			if len(r) > 0 {
				m.query = string(r[:len(r)-1])
			}
		case "ctrl+u":
			m.query = ""
		default:
			if k.Text != "" {
				m.query = fit(Clean(m.query+k.Text), 128)
			}
		}
		m.ensureProcess()
		return nil
	}
	if key == "q" {
		return tea.Quit
	}
	if key == "?" {
		m.help = true
		m.drag = nil
		m.helpOffset = 0
		return nil
	}
	if m.target == nil && !m.expanded && m.activeWindow() < 0 {
		switch key {
		case "shift+right", "shift+left", "shift+up", "shift+down":
			dx, dy := 0, 0
			switch key {
			case "shift+right":
				dx = 1
			case "shift+left":
				dx = -1
			case "shift+down":
				dy = 1
			case "shift+up":
				dy = -1
			}
			m.resizeCard(m.selected, dx, dy)
			return nil
		case "s":
			if m.reordering {
				m.sortCards()
				return nil
			}
		case "0":
			m.selected = 0
			m.order = defaultOrder()
			for i := range m.spans {
				m.spans[i] = Span{1, 1}
			}
			m.reveal()
			return nil
		}
	}
	if m.reordering {
		switch key {
		case "o":
			m.reordering = false
			m.organizing = true
			m.organizerIndex = m.slot(m.selected)
		case "esc":
			m.order = m.reorderOrigin
			m.spans = m.reorderSpans
			m.selected = m.reorderSelected
			m.reordering = false
		case "enter", "p":
			m.reordering = false
		case "up", "k", "down", "j", "left", "h", "right", "l":
			d := 1
			if key == "up" || key == "k" || key == "left" || key == "h" {
				d = -1
			}
			dx, dy := d, 0
			if key == "up" || key == "k" || key == "down" || key == "j" {
				dx, dy = 0, d
			}
			m.moveCard(m.selected, m.layout().neighbor(m.slot(m.selected), dx, dy))
		}
		m.reveal()
		return nil
	}
	if m.target != nil {
		switch key {
		case "esc":
			m.target = nil
			m.inspection = nil
			m.modalOffset = 0
			m.status = "Process list"
		case "x":
			if m.opts.ReadOnly {
				m.status = "Read-only mode: termination disabled"
			} else if m.inspection != nil {
				m.confirm = true
				m.drag = nil
			}
		case "up", "k":
			m.modalOffset--
			m.clampOffsets()
		case "down", "j":
			m.modalOffset++
			m.clampOffsets()
		case "pgdown":
			m.modalOffset += 5
			m.clampOffsets()
		case "pgup":
			m.modalOffset -= 5
			m.clampOffsets()
		}
		return nil
	}
	switch key {
	case "space":
		m.paused = !m.paused
		m.tickGeneration++
		m.status = "Sampling paused"
		if !m.paused {
			m.status = "Sampling resumed"
		}
		if !m.paused {
			return m.sample()
		}
		return nil
	case "H", "delete":
		m.hideSelectedPanel()
	case "r":
		m.refreshOnce = true
		m.status = ""
		return m.sample()
	case "+", "=":
		m.opts.Interval = min(10*time.Second, m.opts.Interval+250*time.Millisecond)
	case "-":
		m.opts.Interval = max(250*time.Millisecond, m.opts.Interval-250*time.Millisecond)
	case "esc":
		if i := m.activeWindow(); i >= 0 {
			m.dock(m.windows[i].Panel)
			return nil
		}
		m.expanded = false
		m.focusWindow = -1
		m.offset = 0
		m.reveal()
		m.status = "Dashboard"
		if len(m.windows) > 0 {
			m.raise(len(m.windows) - 1)
		}
	case "tab", "shift+tab":
		if m.activeWindow() >= 0 {
			if len(m.windows) == 1 {
				return nil
			}
			index := 0
			if key == "shift+tab" {
				index = len(m.windows) - 2
			}
			m.raise(index)
			return nil
		}
		d := 1
		if key == "shift+tab" {
			d = -1
		}
		visible := m.visiblePanels()
		for i, id := range visible {
			if id == m.selected {
				m.selected = visible[(i+d+len(visible))%len(visible)]
				break
			}
		}
		m.offset = 0
		m.reveal()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := m.activeWindow(); i >= 0 {
			m.windows[i].Offset = m.offset
		}
		m.focusWindow = -1
		m.selected = int(key[0] - '1')
		m.expanded = true
		m.offset = 0
		m.ensureProcess()
	case "enter":
		if m.activeWindow() >= 0 {
			if m.kind(m.selected) == 4 {
				return m.openProcess()
			}
			m.maximizeWindow()
			return nil
		}
		if !m.expanded {
			m.expanded = true
			m.offset = 0
			m.ensureProcess()
		} else if m.kind(m.selected) == 4 {
			return m.openProcess()
		}
	case "/":
		if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) == 4 {
			m.editing = true
		}
	case "c":
		if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) == 4 {
			m.query = ""
			m.ensureProcess()
		}
	case "s":
		if m.kind(m.selected) == 4 {
			choices := []string{"CPU", "Memory", "PID", "Name"}
			for i, s := range choices {
				if s == m.sortBy {
					m.sortBy = choices[(i+1)%len(choices)]
					m.sortReverse = false
					break
				}
			}
			m.ensureProcess()
		}
	case "home":
		if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) == 4 {
			m.chooseProcess(0)
		}
		m.offset = 0
	case "end":
		if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) == 4 {
			m.chooseProcess(len(m.processes()) - 1)
		} else if m.expanded || m.activeWindow() >= 0 {
			m.offset = 1 << 20
			m.clampOffsets()
		}
	case "up", "k", "down", "j", "left", "h", "right", "l", "pgup", "pgdown":
		d := 1
		if key == "up" || key == "k" || key == "left" || key == "h" || key == "pgup" {
			d = -1
		}
		if m.expanded || m.activeWindow() >= 0 {
			if key == "pgup" || key == "pgdown" {
				d *= max(1, m.height-10)
			}
			m.moveDetail(d)
		} else {
			if key != "pgup" && key != "pgdown" {
				dx, dy := d, 0
				if key == "up" || key == "k" || key == "down" || key == "j" {
					dx, dy = 0, d
				}
				m.selected = m.order[m.layout().neighbor(m.slot(m.selected), dx, dy)]
				m.reveal()
			}
		}
	case "f":
		m.floatPanel(m.selected)
	case "o":
		m.organizing = true
		m.organizerIndex = m.slot(m.selected)
	case "p":
		if !m.expanded && m.activeWindow() < 0 {
			m.reordering = true
			m.reorderOrigin = m.order
			m.reorderSpans = m.spans
			m.reorderSelected = m.selected
		}
	case "t":
		m.picking = true
		m.pickerIndex = m.kind(m.selected)
		m.drag = nil
	case "g":
		for _, id := range m.order {
			if m.kind(id) == 6 {
				m.selected, m.focusWindow, m.offset = id, -1, 0
				m.expanded = true
				return nil
			}
		}
		m.picking, m.pickerIndex = true, 6
	case "m":
		m.maximizeWindow()
	case "d":
		m.windows = nil
		m.focusWindow = -1
		m.expanded = false
		m.offset = 0
		m.drag = nil
		m.reveal()
		m.status = "All panels docked in the responsive grid"
	}
	return nil
}
