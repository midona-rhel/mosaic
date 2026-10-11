package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestViewportHidesOverflowAndRestoresPriority(t *testing.T) {
	m := testModel()
	m.kinds[0] = 1
	m.SetSize(120, 30)
	saved := m.Workspace()
	m.resizeCard(0, 1, 1)
	if got := len(m.visiblePanels()); got != 5 {
		t.Fatalf("got %d visible panels", got)
	}
	if m.cardRect(5).W != 0 || m.cardRect(6).W != 0 {
		t.Fatal("overflow was not hidden")
	}
	m.SetSize(120, 56)
	if len(m.visiblePanels()) != defaultPanelCount {
		t.Fatal("taller window failed to add grid rows and restore panels")
	}
	m.SetSize(120, 30)
	before := m.layout()
	m.Update(tea.MouseWheelMsg{X: 0, Y: 0, Button: tea.MouseWheelDown})
	press(m, tea.KeyPgDown, "")
	if m.layout().Cards[0] != before.Cards[0] {
		t.Fatal("dashboard scrolled")
	}
	press(m, '7', "7")
	if !m.expanded || m.selected != 6 {
		t.Fatal("hidden panel is not accessible by shortcut")
	}
	press(m, tea.KeyEscape, "")
	if m.selected != 0 {
		t.Fatal("hidden selection did not return to highest priority")
	}
	m.resizeCard(0, -1, -1)
	if m.Workspace() != saved || len(m.visiblePanels()) != defaultPanelCount {
		t.Fatal("shrinking lost panel order or failed to restore hidden panels")
	}
	m.SetSize(40, 16)
	if len(m.visiblePanels()) != 1 {
		t.Fatal("small terminal did not retain only fitting panels")
	}
	m.SetSize(120, 30)
	if m.Workspace() != saved || len(m.visiblePanels()) != defaultPanelCount {
		t.Fatal("terminal resize lost requested layout")
	}
}

func TestQuietHeaderAndDistinctEdgeHover(t *testing.T) {
	m := testModel()
	m.opts.Demo = false
	m.SetSize(120, 30)
	if strings.Contains(ansi.Strip(m.header()), "LIVE") || strings.Contains(ansi.Strip(m.header()), "1s") {
		t.Fatal("routine live/interval badge remains")
	}
	r := m.cardRect(0)
	m.motion(tea.MouseMotionMsg{X: r.X, Y: r.Y + 3})
	hr, e := m.hoveredEdge()
	if hr != r || e != resizeLeft || m.selected != 0 {
		t.Fatal("edge hover changed focus or missed border")
	}
	if !onHighlightedEdge(hr, e, r.X, r.Y+4) || onHighlightedEdge(hr, e, r.X+r.W-1, r.Y+4) {
		t.Fatal("hover colored an unrelated edge")
	}
	if !strings.Contains(m.render(), "115;210;255") {
		t.Fatal("edge hover color missing")
	}
	m.help = true
	_, e = m.hoveredEdge()
	if e != 0 {
		t.Fatal("modal allowed border hover through it")
	}
}
