package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestCardDragRearrangesAndStableHotkeysSurviveResize(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	from, to := m.cardRect(1), m.cardRect(4)
	m.click(mouse(from.X+8, from.Y+1))
	m.motion(tea.MouseMotionMsg{X: to.X + 8, Y: to.Y + 2, Button: tea.MouseLeft})
	m.release(tea.MouseReleaseMsg{X: to.X + 8, Y: to.Y + 2, Button: tea.MouseLeft})
	want := [panelCount]int{0, 2, 3, 4, 1, 5, 6, 7, 8, 9, 10}
	if m.order != want || m.selected != 1 || len(m.windows) != 0 || m.expanded {
		t.Fatalf("drag result: order %+v, selected %d", m.order, m.selected)
	}
	// Navigation follows screen order, while numeric keys retain panel identity.
	press(m, tea.KeyUp, "")
	if m.selected != 0 {
		t.Fatal("focus ignored reordered screen positions")
	}
	m.SetSize(40, 16)
	if m.order != want {
		t.Fatal("resize reset the workspace")
	}
	press(m, '5', "5")
	if m.selected != 4 || !m.expanded {
		t.Fatal("numeric hotkey changed meaning after rearrangement")
	}
	assertFits(t, m)
}

func TestRearrangeCancelAndKeyboardCommit(t *testing.T) {
	m := testModel()
	origin := m.order
	r := m.cardRect(0)
	m.click(mouse(r.X+8, r.Y+1))
	m.motion(tea.MouseMotionMsg{X: 70, Y: 20, Button: tea.MouseLeft})
	press(m, tea.KeyEscape, "")
	m.release(tea.MouseReleaseMsg{X: 70, Y: 20, Button: tea.MouseLeft})
	if m.order != origin || m.expanded {
		t.Fatal("Esc failed to cancel title drag")
	}
	press(m, 'p', "p")
	press(m, tea.KeyRight, "")
	if m.order == origin || !m.reordering {
		t.Fatal("keyboard rearrange didn't move the selected card")
	}
	press(m, tea.KeyEscape, "")
	if m.order != origin || m.reordering {
		t.Fatal("keyboard cancel didn't restore order")
	}
	press(m, 'p', "p")
	press(m, tea.KeyDown, "")
	press(m, tea.KeyEnter, "")
	if m.slot(0) != 3 || m.reordering || m.expanded {
		t.Fatal("keyboard commit didn't preserve the moved card")
	}
	press(m, tea.KeyTab, "")
	if m.selected != 4 {
		t.Fatal("Tab didn't follow workspace order")
	}
}

func TestCardDropIgnoresFloatingWindowsAndOutsideViewport(t *testing.T) {
	for _, point := range [][2]int{{60, 14}, {60, 29}, {119, 5}} {
		m := testModel()
		m.SetSize(120, 30)
		m.floatPanel(1)
		m.focusWindow = -1
		origin := m.order
		r := m.cardRect(0)
		m.click(mouse(r.X+8, r.Y+1))
		m.motion(tea.MouseMotionMsg{X: point[0], Y: point[1], Button: tea.MouseLeft})
		m.release(tea.MouseReleaseMsg{X: point[0], Y: point[1], Button: tea.MouseLeft})
		if m.order != origin {
			t.Fatal("drop through a window or outside a card changed the grid")
		}
	}
}

func TestPopoutHoverAndControlsUseRealHitTargets(t *testing.T) {
	m := testModel()
	m.SetSize(120, 35)
	a := m.cardPopout(2)
	m.motion(tea.MouseMotionMsg{X: a.Rect.X + 1, Y: a.Rect.Y})
	if !strings.Contains(m.hoverHint(), "Pop out Memory") {
		t.Fatal("pop-out button lacks its hover hint")
	}
	if len(m.windows) != 0 {
		t.Fatal("hover unexpectedly opened a window")
	}
	m.click(mouse(a.Rect.X+1, a.Rect.Y))
	if len(m.windows) != 1 || m.focusWindow != 2 || m.drag != nil || m.expanded {
		t.Fatal("pop-out went through ordinary title click handling")
	}
	m.click(mouse(a.Rect.X+1, a.Rect.Y))
	if len(m.windows) != 1 {
		t.Fatal("pop-out created duplicate panels")
	}
	w := m.windows[0]
	controls := m.windowActions(w)
	m.motion(tea.MouseMotionMsg{X: controls[0].Rect.X + 1, Y: controls[0].Rect.Y})
	if !strings.Contains(m.hoverHint(), "Maximize Memory") {
		t.Fatal("window maximize doesn't hover")
	}
	m.click(mouse(controls[0].Rect.X+1, controls[0].Rect.Y))
	if !m.expanded || len(m.windows) != 0 {
		t.Fatal("window maximize failed")
	}
	controls = m.expandedActions()
	m.click(mouse(controls[0].Rect.X+1, controls[0].Rect.Y))
	if m.expanded || len(m.windows) != 1 {
		t.Fatal("expanded view pop-out failed")
	}
	controls = m.windowActions(m.windows[0])
	m.click(mouse(controls[1].Rect.X+1, controls[1].Rect.Y))
	if len(m.windows) != 0 {
		t.Fatal("window dock failed")
	}
}

func TestInspectorAndConfirmationHaveWorkingLocalActions(t *testing.T) {
	for _, width := range []int{24, 80, 120} {
		m := testModel()
		m.SetSize(width, 30)
		m.Update(m.openProcess()())
		as := m.inspectorActions()
		if len(as) != 2 {
			t.Fatal("inspector lacks its local action")
		}
		a := as[1]
		m.click(mouse(a.Rect.X+1, a.Rect.Y))
		if !m.confirm {
			t.Fatal("local terminate didn't request confirmation")
		}
		for _, a := range m.confirmationActions() {
			r := m.confirmationRect()
			if !r.Contains(a.Rect.X, a.Rect.Y) || !r.Contains(a.Rect.X+a.Rect.W-1, a.Rect.Y) {
				t.Fatalf("confirmation action escaped narrow dialog: %+v %+v", r, a)
			}
		}
		a = m.confirmationActions()[0]
		m.click(mouse(a.Rect.X+1, a.Rect.Y))
		if m.confirm || m.target == nil {
			t.Fatal("local cancel did not keep inspector open")
		}
		a = m.inspectorActions()[0]
		m.click(mouse(a.Rect.X+1, a.Rect.Y))
		if m.target != nil {
			t.Fatal("inspector close button didn't close")
		}
	}
}

func TestViewsOmitHealthLegendAndRoutineStatus(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	check := func() {
		if strings.Contains(ansi.Strip(m.header()), "BUSY") || strings.Contains(ansi.Strip(m.header()), "HIGH") {
			t.Fatal("health legend remains")
		}
		assertFits(t, m)
	}
	check()
	m.floatPanel(1)
	check()
	m.Update(m.openProcess()())
	check()
	m.confirm = true
	check()
	if strings.Contains(ansi.Strip(m.footer()), "Inspecting") || strings.Contains(ansi.Strip(m.footer()), "DEMO") {
		t.Fatal("routine bottom-left status text remains")
	}
}

func TestMeterKeepsUsefulFillWhenColorsAreStripped(t *testing.T) {
	plain := ansi.Strip(progressBar(50, 12, false))
	if plain != "[█████░░░░░]" {
		t.Fatalf("monochrome gauge lost its fill: %q", plain)
	}
}
