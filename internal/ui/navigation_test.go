package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDashboardArrowsFollowSpatialNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name               string
		width, start, want int
		key                rune
	}{
		{"four columns right edge", 120, 3, 3, tea.KeyRight},
		{"four columns left edge", 120, 4, 4, tea.KeyLeft},
		{"four columns bottom edge", 120, 5, 5, tea.KeyDown},
		{"four columns vertical", 120, 1, 5, tea.KeyDown},
		{"three columns right edge", 80, 2, 2, tea.KeyRight},
		{"three columns left edge", 80, 3, 3, tea.KeyLeft},
		{"three columns vertical", 80, 4, 1, tea.KeyUp},
		{"one column horizontal", 40, 2, 2, tea.KeyRight},
		{"one column vertical", 40, 2, 1, tea.KeyUp},
		{"top edge", 80, 1, 1, tea.KeyUp},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel()
			m.SetSize(tc.width, 24)
			// Panel identity differs from position after rearranging.
			m.order = [panelCount]int{3, 0, 5, 2, 1, 4, 6, 7, 8, 9, 10}
			m.selected = m.order[tc.start]
			press(m, tc.key, "")
			if m.selected != m.order[tc.want] || m.expanded {
				t.Fatalf("selected %d, want panel at slot %d", m.selected, tc.want)
			}
			r := m.cardRect(m.selected)
			if r.Y < 2 || r.Y+r.H > m.height-2 {
				t.Fatal("keyboard focus is outside the visible dashboard")
			}
			press(m, tea.KeyEnter, "")
			if !m.expanded || m.selected != m.order[tc.want] {
				t.Fatal("Enter did not open the focused card")
			}
			press(m, tea.KeyEscape, "")
			if m.expanded || m.selected != m.order[tc.want] {
				t.Fatal("Esc did not return to the focused card")
			}
		})
	}
}

func TestNavigationHintsAreVisibleAndDoNotCaptureClicks(t *testing.T) {
	m := testModel()
	for _, width := range []int{80, 120} {
		m.SetSize(width, 24)
		as, line := m.actionBar()
		if !strings.Contains(ansi.Strip(line), "↑↓←→ move") {
			t.Fatal("dashboard arrow navigation is hidden")
		}
		for _, a := range as {
			if a.Informational || a.Key.Code == 0 {
				t.Fatal("navigation legend became an unbound click target")
			}
		}
	}
	press(m, '5', "5")
	_, line := m.actionBar()
	if !strings.Contains(ansi.Strip(line), "↑↓ select") {
		t.Fatal("process navigation hint is missing")
	}
	before := m.selectedPID
	press(m, tea.KeyDown, "")
	if m.selectedPID == before {
		t.Fatal("Down did not select another process")
	}
	press(m, tea.KeyUp, "")
	if m.selectedPID != before {
		t.Fatal("Up did not restore the previous selection")
	}
}
