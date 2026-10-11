package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Workspace stores card identities separately from their content and position.
// Ephemeral inspectors, filters and destructive confirmations are never restored.
type Workspace struct {
	Version  int              `json:"version"`
	Order    [panelCount]int  `json:"order"`
	Kinds    [panelCount]int  `json:"kinds"`
	Spans    [panelCount]Span `json:"spans"`
	Disabled [panelCount]bool `json:"disabled"`
}

func (m *Model) Workspace() Workspace { return Workspace{2, m.order, m.kinds, m.spans, m.disabled} }
func (m *Model) ApplyWorkspace(p Workspace) error {
	if p.Version == 1 {
		// Preserve all seven legacy cards and their requested sizes. Add Disk
		// activity after them; the three optional panels start removed.
		for i := 7; i < panelCount; i++ {
			p.Order[i] = i
			p.Kinds[i] = i
			p.Spans[i] = Span{1, 1}
			p.Disabled[i] = i >= defaultPanelCount
		}
		p.Version = 2
	}
	if p.Version != 2 {
		return fmt.Errorf("unsupported layout version %d", p.Version)
	}
	seen := map[int]bool{}
	for i, id := range p.Order {
		if id < 0 || id >= panelCount || seen[id] {
			return fmt.Errorf("invalid panel order")
		}
		seen[id] = true
		if p.Kinds[i] < 0 || p.Kinds[i] >= panelCount {
			return fmt.Errorf("invalid panel type")
		}
		if p.Spans[i].W < 1 || p.Spans[i].W > 16 || p.Spans[i].H < 1 || p.Spans[i].H > 16 {
			return fmt.Errorf("invalid panel size")
		}
	}
	m.order, m.kinds, m.spans, m.disabled = p.Order, p.Kinds, p.Spans, p.Disabled
	m.reveal()
	return nil
}
func (m *Model) LoadWorkspace(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 65536 {
		return fmt.Errorf("layout exceeds 64 KiB")
	}
	var p Workspace
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	return m.ApplyWorkspace(p)
}
func (m *Model) SaveWorkspace(path string) error {
	p := m.Workspace()
	if m.reordering {
		p.Order, p.Spans = m.reorderOrigin, m.reorderSpans
	}
	if m.drag != nil && m.drag.Kind == "card-resize" {
		p.Spans[m.drag.Panel] = m.drag.SpanOrigin
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mosaic-layout-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// OpenPanel also supports deterministic screenshots of every resource view.
func (m *Model) OpenPanel(name string) error {
	name = strings.ReplaceAll(name, "-", " ")
	if name == "disk io" {
		name = "Disk activity"
	}
	if name == "" {
		return nil
	}
	for kind, title := range panels {
		if strings.EqualFold(name, title) || (kind == 3 && name == "disk") || (kind == 4 && name == "process") {
			for id := range m.kinds {
				if m.kind(id) == kind {
					m.selected = id
					m.expanded = true
					m.offset = 0
					m.ensureProcess()
					return nil
				}
			}
			return fmt.Errorf("panel %q is not in this workspace", name)
		}
	}
	return fmt.Errorf("unknown panel %q; choose system, cpu, memory, disks, processes, network, gpu, disk activity, battery, thermals or connections", name)
}
func (m *Model) pickerRect() Rect {
	w, h := min(56, m.width-4), min(panelCount+5, m.height-4)
	return Rect{(m.width - w) / 2, (m.height - h) / 2, w, h}
}
func (m *Model) pickerStart() int { return max(0, m.pickerIndex-max(1, m.pickerRect().H-5)+1) }
func (m *Model) pickerLines(w int) []string {
	lines := []string{strong.Render("CHOOSE PANEL"), dim.Render("Replace this card · duplicates allowed"), ""}
	for i := m.pickerStart(); i < panelCount && len(lines) < m.pickerRect().H-2; i++ {
		label := fmt.Sprintf("  %d  %s", i+1, panels[i])
		if i == m.pickerIndex {
			label = strong.Foreground(accent).Render(fit("›"+label[1:], w))
		}
		lines = append(lines, label)
	}
	return lines
}
func (m *Model) pickerKey(k tea.KeyPressMsg) tea.Cmd {
	switch key := k.String(); key {
	case "esc", "t":
		m.picking = false
	case "up", "k":
		m.pickerIndex = (m.pickerIndex + panelCount - 1) % panelCount
	case "down", "j":
		m.pickerIndex = (m.pickerIndex + 1) % panelCount
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.pickerIndex = int(key[0] - '1')
	case "enter":
		m.kinds[m.selected] = m.pickerIndex
		m.picking, m.editing = false, false
		m.offset = 0
		if i := m.activeWindow(); i >= 0 {
			m.windows[i].Offset = 0
		}
		m.ensureProcess()
	}
	return nil
}
func (m *Model) pickerClick(v tea.MouseClickMsg) tea.Cmd {
	as, _ := m.actionBar()
	for _, a := range as {
		if a.Rect.Contains(v.X, v.Y) {
			return m.pickerKey(a.Key)
		}
	}
	r := m.pickerRect()
	if !r.Contains(v.X, v.Y) {
		m.picking = false
		return nil
	}
	i := v.Y - r.Y - 4 + m.pickerStart()
	if v.Y >= r.Y+4 && v.Y < r.Y+r.H-1 && i >= 0 && i < panelCount {
		m.pickerIndex = i
		return m.pickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	return nil
}

// Keep long identifiers and paths accessible instead of silently truncating them.
func detailPair(label, value string, w int) []string {
	value = strings.TrimSpace(Clean(value))
	if value == "" {
		value = "Unavailable"
	}
	if ansi.StringWidth(label)+ansi.StringWidth(value)+2 <= w {
		return []string{pair(dim.Render(label), value, w)}
	}
	return append([]string{dim.Render(label)}, strings.Split(ansi.Hardwrap(value, max(1, w), true), "\n")...)
}
