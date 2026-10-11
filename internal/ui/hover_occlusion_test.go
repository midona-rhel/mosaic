package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCoveredControlsDoNotHighlight(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	card := m.cardPopout(0)
	m.floatPanel(1)
	// Cover only the right part of the card button. Its exposed left part
	// must remain unchanged when the mouse is on the covering window.
	m.windows[0].Bounds = Rect{card.Rect.X + 1, 2, 40, 18}
	m.motion(tea.MouseMotionMsg{X: card.Rect.X + 1, Y: card.Rect.Y})
	if strings.Contains(m.control(card), "80;53;117") {
		t.Fatal("covered dashboard control received hover color")
	}
	m.motion(tea.MouseMotionMsg{X: card.Rect.X, Y: card.Rect.Y})
	if !strings.Contains(m.control(card), "80;53;117") {
		t.Fatal("exposed dashboard control lost hover")
	}
	lower := m.windowActions(m.windows[0])[0]
	m.floatPanel(2)
	m.windows[1].Bounds = Rect{lower.Rect.X + 1, 2, 40, 18}
	m.motion(tea.MouseMotionMsg{X: lower.Rect.X + 1, Y: lower.Rect.Y})
	if strings.Contains(m.control(lower), "80;53;117") {
		t.Fatal("covered floating-window control received hover color")
	}
	upper := m.windowActions(m.windows[1])[0]
	m.motion(tea.MouseMotionMsg{X: upper.Rect.X, Y: upper.Rect.Y})
	if !strings.Contains(m.control(upper), "80;53;117") {
		t.Fatal("topmost control lost hover")
	}
	m.help = true
	if strings.Contains(m.control(upper), "80;53;117") {
		t.Fatal("modal allowed control hover underneath")
	}
}

func TestEdgeHoverDoesNotPaintThroughWindow(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	r := m.cardRect(0)
	m.floatPanel(1)
	m.windows[0].Bounds = Rect{r.X, r.Y + 4, 40, 16}
	m.motion(tea.MouseMotionMsg{X: r.X, Y: r.Y + 2})
	lines := strings.Split(m.render(), "\n")
	if !strings.Contains(lines[r.Y+2], "115;210;255") {
		t.Fatal("exposed hovered edge did not highlight")
	}
	if strings.Contains(lines[r.Y+5], "115;210;255") {
		t.Fatal("dashboard edge highlighted through floating window")
	}
	m.motion(tea.MouseMotionMsg{X: r.X + 1, Y: r.Y + 6})
	_, edge := m.hoveredEdge()
	if edge != 0 || m.hoverHint() != "" || m.pointerShape() != "default" {
		t.Fatal("floating content leaked hover to the dashboard")
	}
}
