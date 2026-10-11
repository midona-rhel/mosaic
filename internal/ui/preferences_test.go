package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestEightDefaultPanelsVisibleAtStandardSizes(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 30}} {
		m := testModel()
		m.SetSize(size[0], size[1])
		for id := range defaultPanelCount {
			r := m.cardRect(id)
			if r.Y < 2 || r.Y+r.H > m.height-2 {
				t.Fatalf("panel %d hidden at %v", id, size)
			}
			if !strings.Contains(ansi.Strip(m.View().Content), strings.ToUpper(m.cardTitle(id))) {
				t.Fatalf("missing title %s at %v", panels[id], size)
			}
		}
	}
}
func TestReplacementRoutesAllViewsAndActionsByContent(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	press(m, 't', "t")
	press(m, '5', "5")
	press(m, tea.KeyEnter, "")
	if m.kind(0) != 4 || m.kind(4) != 4 {
		t.Fatal("duplicate process panel wasn't created")
	}
	press(m, tea.KeyEnter, "")
	if !strings.Contains(ansi.Strip(m.View().Content), "Filter:") {
		t.Fatal("replacement did not render process table")
	}
	press(m, '/', "/")
	if !m.editing {
		t.Fatal("replacement lost process actions")
	}
	press(m, tea.KeyEscape, "")
	press(m, tea.KeyEnter, "")
	if m.target == nil {
		t.Fatal("replacement cannot inspect processes")
	}
	press(m, tea.KeyEscape, "")
	press(m, 't', "t")
	press(m, '7', "7")
	press(m, tea.KeyEnter, "")
	if m.kind(0) != 6 || m.kind(6) != 6 {
		t.Fatal("GPU replacement failed")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "63.0%") {
		t.Fatal("GPU values missing in replacement")
	}
	press(m, 'f', "f")
	if !strings.Contains(ansi.Strip(m.View().Content), ":: GPU") {
		t.Fatal("replacement pop-out title wrong")
	}
}
func TestPickerModalMouseCancellationAndTinyBounds(t *testing.T) {
	for _, size := range [][2]int{{24, 10}, {80, 24}, {120, 30}} {
		m := testModel()
		m.SetSize(size[0], size[1])
		press(m, 't', "t")
		press(m, '7', "7")
		assertFits(t, m)
		before := m.Workspace()
		m.click(mouse(0, 0))
		if m.picking || m.Workspace() != before {
			t.Fatal("outside picker click mutated layout")
		}
		press(m, 't', "t")
		press(m, '7', "7")
		r := m.pickerRect()
		row := m.pickerIndex - m.pickerStart()
		m.click(mouse(r.X+3, r.Y+4+row))
		if m.picking || m.kind(0) != 6 {
			t.Fatal("mouse choice did not match highlighted row")
		}
	}
}
func TestWorkspacePersistenceAndInvalidConfigIsAtomic(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.moveCard(1, 4)
	m.spans[2] = Span{2, 2}
	m.kinds[0] = 6
	if err := m.SetMeterPreference("fira"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config", "layout.json")
	if err := m.SaveWorkspace(path); err != nil {
		t.Fatal(err)
	}
	n := testModel()
	if err := n.LoadWorkspace(path); err != nil {
		t.Fatal(err)
	}
	if n.Workspace() != m.Workspace() {
		t.Fatal("workspace not restored")
	}
	n.SetSize(40, 16)
	n.SetSize(120, 30)
	if n.Workspace() != m.Workspace() {
		t.Fatal("narrow terminal erased preferences")
	}
	original := n.Workspace()
	badMeters := original
	badMeters.Meters = "unknown-font"
	if err := n.ApplyWorkspace(badMeters); err == nil || n.Workspace() != original {
		t.Fatal("invalid meter setting changed the workspace")
	}
	for _, bad := range []Workspace{{}, {Version: 99}, {Version: 1, Order: [panelCount]int{0, 0, 2, 3, 4, 5, 6}}, {Version: 1, Order: original.Order, Kinds: original.Kinds, Spans: [panelCount]Span{}}} {
		if err := n.ApplyWorkspace(bad); err == nil {
			t.Fatal("invalid workspace accepted")
		}
		if n.Workspace() != original {
			t.Fatal("invalid workspace partially applied")
		}
	}
	if err := os.WriteFile(path, []byte("{ broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if n.LoadWorkspace(path) == nil {
		t.Fatal("corrupt config accepted")
	}
	// A second save must replace the existing file, including on Windows CI.
	if err := m.SaveWorkspace(path); err != nil {
		t.Fatal(err)
	}
}
func TestUncommittedLayoutIsNotPersisted(t *testing.T) {
	m := testModel()
	before := m.Workspace()
	press(m, 'p', "p")
	shifted(m, tea.KeyRight)
	press(m, 's', "s")
	path := filepath.Join(t.TempDir(), "layout.json")
	if err := m.SaveWorkspace(path); err != nil {
		t.Fatal(err)
	}
	n := testModel()
	if err := n.LoadWorkspace(path); err != nil {
		t.Fatal(err)
	}
	if n.Workspace() != before {
		t.Fatal("unfinished layout transaction persisted")
	}
}
func TestLongDetailsAreWrappedWithoutLosingValues(t *testing.T) {
	value := "/a/long/path/to/a/device/with/a/very-long-identifier"
	got := ansi.Strip(strings.Join(detailPair("Device", value, 24), ""))
	if !strings.Contains(got, value) {
		t.Fatal("detail value silently truncated")
	}
}

func TestManualRefreshWhilePausedUpdatesOnce(t *testing.T) {
	m := testModel()
	press(m, ' ', " ")
	cmd := press(m, 'r', "r")
	if cmd == nil {
		t.Fatal("refresh unavailable")
	}
	old := m.snapshot.At
	msg := cmd().(sampleMsg)
	msg.At = old.Add(1)
	_, next := m.Update(msg)
	if !m.snapshot.At.After(old) || !m.paused || next != nil {
		t.Fatal("refresh did not update once while preserving pause")
	}
}
