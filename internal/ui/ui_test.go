package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
)

func testModel() *Model { return New(context.Background(), Options{Demo: true}) }
func press(m *Model, code rune, text string) tea.Cmd {
	return m.key(tea.KeyPressMsg{Code: code, Text: text})
}
func TestGridInvariants(t *testing.T) {
	for w := 24; w <= 320; w++ {
		for h := 10; h <= 85; h++ {
			l := Grid(w, h)
			for i, r := range l.Cards {
				if r.W == 0 {
					continue
				}
				if r.W != l.CardWidth || r.H != l.CardHeight || r.X < 0 || r.X+r.W > w {
					t.Fatalf("invalid card %d at %dx%d: %+v", i, w, h, r)
				}
				for j, q := range l.Cards[:i] {
					if r.X < q.X+q.W && r.X+r.W > q.X && r.Y < q.Y+q.H && r.Y+r.H > q.Y {
						t.Fatalf("cards %d and %d overlap at %dx%d", i, j, w, h)
					}
				}
				if r.Y < 2 || r.Y+r.H > h-2 {
					t.Fatalf("card %d exceeds viewport at %dx%d", i, w, h)
				}
			}
		}
	}
	if l := Grid(80, 24); l.Columns != 3 {
		t.Fatalf("80×24 must fit all seven: %+v", l)
	}
}
func assertFits(t *testing.T, m *Model) {
	t.Helper()
	s := ansi.Strip(m.View().Content)
	lines := strings.Split(s, "\n")
	if len(lines) > m.height {
		t.Fatalf("%dx%d: rendered %d lines", m.width, m.height, len(lines))
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("%dx%d: line %d overflow: %q", m.width, m.height, i, line)
		}
	}
}
func TestEveryViewFitsAfterResize(t *testing.T) {
	for _, w := range []int{1, 23, 24, 40, 67, 68, 80, 101, 102, 120, 180} {
		for _, h := range []int{1, 9, 10, 12, 24, 30, 48} {
			m := testModel()
			m.SetSize(w, h)
			assertFits(t, m)
			if w < 24 || h < 10 {
				continue
			}
			for i := range panelCount {
				m.selected = i
				m.expanded = true
				assertFits(t, m)
				m.moveDetail(10000)
				assertFits(t, m)
				m.offset = 0
			}
			m.help = true
			assertFits(t, m)
			m.helpOffset = 10
			assertFits(t, m)
			m.help = false
			p := metrics.Demo().Processes[0]
			m.target = &p
			m.inspection = &metrics.Detail{Process: p, Command: strings.Repeat("A界", 300)}
			assertFits(t, m)
			m.modalOffset = 1000
			m.clampOffsets()
			assertFits(t, m)
			m.confirm = true
			assertFits(t, m)
			if h == 10 && !strings.Contains(ansi.Strip(m.View().Content), "y terminate") {
				t.Fatal("confirmation choice hidden in small terminal")
			}
		}
	}
}
func TestMouseSelectsCardAndExactProcessRow(t *testing.T) {
	m := testModel()
	l := Grid(80, 24)
	r := l.Cards[4]
	m.click(tea.MouseClickMsg{X: r.X + 2, Y: r.Y + 2, Button: tea.MouseLeft})
	if m.expanded || m.selected != 4 {
		t.Fatal("process card click must select without opening")
	}
	press(m, tea.KeyEnter, "")
	if cmd := m.click(tea.MouseClickMsg{X: 10, Y: 7, Button: tea.MouseLeft}); cmd != nil || m.target != nil {
		t.Fatal("column header must not open a process")
	}
	cmd := m.click(tea.MouseClickMsg{X: 10, Y: 8, Button: tea.MouseLeft})
	if cmd == nil || m.target == nil || m.target.PID != m.processes()[0].PID {
		t.Fatal("first process row should open the process at the top of the selected sort")
	}
	m.Update(cmd())
	if m.inspection == nil {
		t.Fatal("inspection did not arrive")
	}
	press(m, tea.KeyEscape, "")
	if m.target != nil || !m.expanded {
		t.Fatal("Esc should close one level")
	}
	press(m, tea.KeyEscape, "")
	if m.expanded {
		t.Fatal("second Esc should restore dashboard")
	}
}
func TestProcessSelectionSurvivesSortAndRefresh(t *testing.T) {
	m := testModel()
	press(m, '5', "5")
	m.chooseProcess(2)
	pid, start := m.selectedPID, m.selectedStart
	press(m, 's', "s")
	s := metrics.Demo()
	for i := range s.Processes {
		s.Processes[i].CPU = float64(100 - i)
	}
	m.SetSnapshot(s)
	if m.selectedPID != pid || m.selectedStart != start {
		t.Fatal("refresh changed selected identity")
	}
	press(m, '/', "/")
	press(m, 'q', "q")
	if m.query != "q" || !m.editing {
		t.Fatal("search q must not quit")
	}
	m.Update(tea.PasteMsg{Content: "\x1b[31m界\n"})
	if strings.Contains(m.query, "\x1b") || strings.Contains(m.query, "\n") {
		t.Fatal("pasted controls escaped sanitization")
	}
	press(m, tea.KeyEscape, "")
	press(m, 'c', "c")
	if len(m.processes()) != 6 {
		t.Fatal("clear failed")
	}
	// Recycling the same PID must not preserve a stale process identity.
	for i := range s.Processes {
		if s.Processes[i].PID == m.selectedPID {
			s.Processes[i].Started++
		}
	}
	m.SetSnapshot(s)
	p := m.processes()[m.processIndex()]
	if p.Started != m.selectedStart {
		t.Fatal("selected identity stale after PID reuse")
	}
}
func TestRefreshDoesNotCreateMultipleSamplingLoops(t *testing.T) {
	m := testModel()
	cmd := m.Init()
	if m.sample() != nil {
		t.Fatal("overlapping sample allowed")
	}
	_, tick := m.Update(cmd())
	old := m.tickGeneration
	manual := m.sample()
	if manual == nil {
		t.Fatal("manual refresh unavailable")
	}
	if _, cmd := m.Update(tickMsg{old}); cmd != nil {
		t.Fatal("stale timer started another sample")
	}
	m.Update(manual())
	if tick == nil {
		t.Fatal("sample completion did not schedule refresh")
	}
	press(m, ' ', " ")
	if !m.paused {
		t.Fatal("pause failed")
	}
	if _, cmd := m.Update(tickMsg{m.tickGeneration}); cmd != nil {
		t.Fatal("paused timer sampled")
	}
}
func TestReadOnlyAndDemoCannotTerminateRealProcesses(t *testing.T) {
	m := testModel()
	m.opts.ReadOnly = true
	press(m, '5', "5")
	cmd := m.openProcess()
	m.Update(cmd())
	press(m, 'x', "x")
	if m.confirm || m.terminate() != nil {
		t.Fatal("read-only termination enabled")
	}
	m.opts.ReadOnly = false
	press(m, 'x', "x")
	if !m.confirm {
		t.Fatal("confirmation did not open")
	}
	if cmd := press(m, tea.KeyEnter, ""); cmd != nil || m.terminating {
		t.Fatal("Enter must not confirm termination")
	}
	cmd = press(m, 'y', "y")
	msg := cmd().(terminateMsg)
	if msg.Err == nil {
		t.Fatal("demo should refuse real termination")
	}
}
func TestLiveRefreshPreservesDetailScroll(t *testing.T) {
	m := testModel()
	m.expanded = true
	m.selected = 1
	s := metrics.Demo()
	s.CPU = make([]float64, 100)
	m.SetSnapshot(s)
	m.moveDetail(20)
	offset := m.offset
	m.SetSnapshot(s)
	if m.offset != offset {
		t.Fatalf("CPU scroll moved on refresh: %d -> %d", offset, m.offset)
	}
	m.selected = 4
	s.Processes = nil
	for i := range 160 {
		s.Processes = append(s.Processes, metrics.Process{PID: int32(i + 2), Name: fmt.Sprint(i), Started: 123, CPUReady: true})
	}
	m.SetSnapshot(s)
	m.chooseProcess(159)
	m.SetSize(40, 12)
	if m.processIndex() < m.offset || m.processIndex() >= m.offset+m.processVisibleRows() {
		t.Fatal("selected process disappeared on resize")
	}
}
func TestSmallProcessViewStillShowsAndClicksAProcess(t *testing.T) {
	m := testModel()
	m.SetSize(40, 10)
	press(m, '5', "5")
	if !strings.Contains(ansi.Strip(m.View().Content), "firefox") {
		t.Fatal("small process view has no visible row")
	}
	cmd := m.click(tea.MouseClickMsg{X: 10, Y: 6, Button: tea.MouseLeft})
	if cmd == nil || m.target == nil || m.target.PID != 821 {
		t.Fatal("compact process row click failed")
	}
}
func TestUnavailableMetricsStayVisible(t *testing.T) {
	m := testModel()
	m.SetSnapshot(metrics.Snapshot{At: metrics.Demo().At, Problems: map[string]string{"CPU": "permission denied"}})
	press(m, '2', "2")
	if !strings.Contains(ansi.Strip(m.View().Content), "permission denied") {
		t.Fatal("collection error hidden")
	}
	if !strings.Contains(Clean("abc\x1b[31mdef\n"), "abcdef") {
		t.Fatal("sanitizer damaged plain text")
	}
}
