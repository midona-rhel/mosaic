package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBatteryCardShowsRemainingAgainstFull(t *testing.T) {
	m := testModel()
	card := ansi.Strip(strings.Join(m.extraCard(8, 40, 10), "\n"))
	if !strings.Contains(card, "3666 / 4700 mAh") || strings.Contains(card, "94%") {
		t.Fatal("battery card must prioritize remaining/full over design ratio:", card)
	}
	if batteryCapacity(metrics.Battery{}) != "Unavailable" {
		t.Fatal("missing capacity was invented")
	}
	now, full := 24.0, 48.0
	if batteryCapacity(metrics.Battery{Remaining: &now, Full: &full, CapacityUnit: "Wh"}) != "24.0 / 48.0 Wh" {
		t.Fatal("energy units lost")
	}
}

func TestHidePanelControlsPreserveLayoutAndRestore(t *testing.T) {
	for _, mode := range []string{"key", "delete", "card", "footer"} {
		t.Run(mode, func(t *testing.T) {
			m := testModel()
			m.SetSize(120, 30)
			m.selected = 1
			m.spans[1] = Span{2, 1}
			order, spans := m.order, m.spans
			var a action
			switch mode {
			case "key":
				press(m, 'H', "H")
			case "delete":
				press(m, tea.KeyDelete, "")
			case "card":
				a = m.cardHide(1)
			case "footer":
				as, _ := m.actionBar()
				for _, v := range as {
					if v.Key.Code == 'H' {
						a = v
					}
				}
			}
			if mode != "key" && mode != "delete" {
				if a.Rect.W == 0 {
					t.Fatal("missing hide button")
				}
				m.click(mouse(a.Rect.X, a.Rect.Y))
			}
			if !m.disabled[1] || m.expanded || m.windowIndex(1) >= 0 || m.order != order || m.spans != spans || m.drag != nil {
				t.Fatal("hide lost geometry, left a window open, or started a drag")
			}
			press(m, 'o', "o")
			press(m, '2', "2")
			press(m, ' ', " ")
			if m.disabled[1] || m.order != order || m.spans != spans {
				t.Fatal("restore lost arrangement")
			}
		})
	}
}

func TestPanelManagerAddRemoveMoveSortAndPersist(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	if len(m.visiblePanels()) != 8 || m.span(6) != (Span{1, 1}) {
		t.Fatal("default dashboard must have eight equal cards")
	}
	press(m, 'o', "o")
	press(m, '9', "9")
	if !strings.Contains(ansi.Strip(m.render()), "Removed") {
		t.Fatal("optional panels not discoverable")
	}
	press(m, ' ', " ")
	if m.disabled[8] || m.order != defaultOrder() || m.organizerIndex != 8 {
		t.Fatal("add changed saved priority or menu selection")
	}
	m.key(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	if m.order[9] != 8 || m.organizerIndex != 9 {
		t.Fatal("manager failed to move battery")
	}
	press(m, 's', "s")
	if m.order[0] != 8 {
		t.Fatal("alphabetical sort did not place Battery first")
	}
	path := filepath.Join(t.TempDir(), "layout.json")
	if e := m.SaveWorkspace(path); e != nil {
		t.Fatal(e)
	}
	n := testModel()
	if e := n.LoadWorkspace(path); e != nil {
		t.Fatal(e)
	}
	if n.Workspace() != m.Workspace() {
		t.Fatal("added panels or order not persisted")
	}
	press(m, ' ', " ")
	if !m.disabled[8] || m.cardRect(8).W != 0 {
		t.Fatal("remove did not remove battery")
	}
	if strings.Contains(strings.Join(m.previewHidden(), " "), "Battery") {
		t.Fatal("removed panel was reported as overflow")
	}
	for i := range m.disabled {
		m.disabled[i] = true
	}
	press(m, tea.KeyEscape, "")
	assertFits(t, m)
	press(m, 'o', "o")
	press(m, tea.KeyEnter, "")
	if len(m.visiblePanels()) != 1 {
		t.Fatal("empty workspace could not add a panel")
	}
}

func TestPanelTogglePreservesCustomOrderAndSize(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.moveCard(8, 3)
	m.spans[8] = Span{2, 1}
	order, spans := m.order, m.spans
	press(m, 'o', "o")
	press(m, '9', "9")
	for _, key := range []string{"+", "+", "-", "-", " ", " "} {
		press(m, rune(key[0]), key)
		if m.order != order || m.spans != spans || m.organizerIndex != 3 {
			t.Fatalf("toggle %q changed order, size or menu row", key)
		}
	}
	// Disabled state and its original priority must both survive a restart.
	path := filepath.Join(t.TempDir(), "layout.json")
	if err := m.SaveWorkspace(path); err != nil {
		t.Fatal(err)
	}
	n := testModel()
	if err := n.LoadWorkspace(path); err != nil {
		t.Fatal(err)
	}
	press(n, 'o', "o")
	press(n, '9', "9")
	press(n, ' ', " ")
	if n.disabled[8] || n.order != order || n.spans != spans {
		t.Fatal("re-enabling after restart changed saved arrangement")
	}
}
func TestLegacyLayoutMigrationPreservesUserSpans(t *testing.T) {
	p := filepath.Join(t.TempDir(), "old.json")
	data := `{"version":1,"order":[3,0,1,2,4,5,6],"kinds":[0,1,2,3,4,5,6],"spans":[{"W":1,"H":1},{"W":1,"H":1},{"W":1,"H":1},{"W":2,"H":1},{"W":1,"H":1},{"W":1,"H":1},{"W":2,"H":1}]}`
	if e := os.WriteFile(p, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	m := testModel()
	if e := m.LoadWorkspace(p); e != nil {
		t.Fatal(e)
	}
	if m.order[0] != 3 || m.spans[6] != (Span{2, 1}) || m.disabled[7] || !m.disabled[8] || m.Workspace().Version != 2 {
		t.Fatal("legacy preferences were lost")
	}
}
func TestExtraPanelsCompactExpandedAndFloating(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 30}} {
		for kind, want := range map[int]string{7: "Read", 8: "Discharging", 9: "CPU", 10: "sockets"} {
			m := testModel()
			m.SetSize(size[0], size[1])
			m.kinds[0] = kind
			if !strings.Contains(ansi.Strip(strings.Join(m.cardLines(0, 30, 10), "\n")), want) {
				t.Fatalf("panel %d missing data", kind)
			}
			press(m, tea.KeyEnter, "")
			assertFits(t, m)
			if !strings.Contains(ansi.Strip(m.render()), want) {
				t.Fatalf("details %d missing data", kind)
			}
			press(m, 'f', "f")
			assertFits(t, m)
			if len(m.windows) != 1 {
				t.Fatal("new panel did not float")
			}
		}
	}
}
func TestResizeReusesFrameWithinGridStep(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	r := m.cardRect(1)
	p := WithMousePointer(m).(*pointerModel)
	p.Update(mouse(r.X+r.W-1, r.Y+4))
	p.Update(tea.MouseMotionMsg{X: r.X + r.W, Y: r.Y + 4, Button: tea.MouseLeft})
	p.View()
	p.Update(tea.MouseMotionMsg{X: r.X + r.W + 1, Y: r.Y + 4, Button: tea.MouseLeft})
	if !p.valid {
		t.Fatal("unchanged resize proposal repainted the dashboard")
	}
	p.Update(tea.MouseMotionMsg{X: r.X + r.W + 20, Y: r.Y + 4, Button: tea.MouseLeft})
	if !p.framePending {
		t.Fatal("changed resize proposal failed to schedule a redraw")
	}
	p.Update(dragFrameMsg(p.frameGeneration))
	if p.valid {
		t.Fatal("scheduled resize frame did not redraw")
	}
	p.View()
	p.Update(tea.MouseReleaseMsg{X: r.X + r.W + 20, Y: r.Y + 4, Button: tea.MouseLeft})
	if p.valid || m.span(1).W != 2 {
		t.Fatal("release did not commit and redraw")
	}
}

func TestFloatingWindowCannotHideItsGridPanel(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.floatPanel(1)
	for _, a := range append(m.windowActions(m.windows[0]), m.actions()...) {
		if a.Key.Code == 'H' {
			t.Fatal("floating view exposes grid hide action")
		}
	}
	for _, key := range []tea.KeyPressMsg{{Code: 'H', Text: "H"}, {Code: tea.KeyDelete}} {
		m.key(key)
		if m.disabled[1] || m.windowIndex(1) < 0 {
			t.Fatal("floating shortcut changed grid visibility")
		}
	}
	m.maximizeWindow()
	for _, a := range append(m.expandedActions(), m.actions()...) {
		if a.Key.Code == 'H' {
			t.Fatal("expanded view exposes grid hide action")
		}
	}
	press(m, 'H', "H")
	if m.disabled[1] {
		t.Fatal("expanded view hid grid panel")
	}
	m.floatPanel(1)
	a := m.windowActions(m.windows[0])[1]
	m.click(mouse(a.Rect.X, a.Rect.Y))
	if m.disabled[1] || m.windowIndex(1) >= 0 {
		t.Fatal("close should dock without hiding")
	}
}
