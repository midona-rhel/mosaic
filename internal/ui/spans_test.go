package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
)

func shifted(m *Model, code rune) { m.key(tea.KeyPressMsg{Code: code, Mod: tea.ModShift}) }

func checkPacked(t *testing.T, m *Model) {
	t.Helper()
	l := m.layout()
	hidden := false
	for i, r := range l.Cards {
		if r.W == 0 {
			hidden = true
			continue
		}
		if hidden {
			t.Fatal("lower-priority card leapfrogged a hidden card")
		}
		if r.X < 1 || r.X+r.W > m.width-1 || r.Y < 2 || r.Y+r.H > m.height-2 || r.W < l.CardWidth || r.H < l.CardHeight {
			t.Fatalf("invalid panel %d: %+v at %dx%d", i, r, m.width, m.height)
		}
		if (r.W-l.CardWidth)%l.ColumnStep != 0 || (r.H-l.CardHeight)%l.RowStep != 0 {
			t.Fatal("panel left the grid")
		}
		if i > 0 {
			p := l.Cards[i-1]
			if r.Y < p.Y || (r.Y == p.Y && r.X < p.X) {
				t.Fatal("packing changed reading order")
			}
		}
		for _, q := range l.Cards[:i] {
			if r.X < q.X+q.W && r.X+r.W > q.X && r.Y < q.Y+q.H && r.Y+r.H > q.Y {
				t.Fatal("resized panels overlap")
			}
		}
		m.selected = m.order[i]
		m.reveal()
		visible := m.cardRect(m.selected)
		if visible.Y < 2 || visible.Y+visible.H > m.height-2 {
			t.Fatal("resized card cannot be revealed")
		}
	}
	assertFits(t, m)
}

func TestSpansPackOnGridAndSurviveResponsiveReflow(t *testing.T) {
	for _, width := range []int{24, 40, 67, 68, 80, 101, 102, 120, 180} {
		for _, height := range []int{12, 24, 30, 48} {
			m := testModel()
			m.SetSize(width, height)
			m.order = [panelCount]int{3, 1, 4, 0, 5, 2, 6, 7, 8, 9, 10}
			for i := range m.spans {
				m.spans[i] = Span{1 + i%3, 1 + (i/2)%3}
			}
			saved := m.spans
			checkPacked(t, m)
			m.SetSize(40, 16)
			checkPacked(t, m)
			m.SetSize(width, height)
			checkPacked(t, m)
			if m.spans != saved {
				t.Fatal("terminal resize erased requested sizes")
			}
		}
	}
}

func TestKeyboardResizeLayoutSortCancelAndReset(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	order, spans := m.order, m.spans
	m.selected = 1
	press(m, 'p', "p")
	shifted(m, tea.KeyRight)
	shifted(m, tea.KeyDown)
	if m.span(1) != (Span{2, 2}) {
		t.Fatalf("shift arrows did not grow independently: %+v", m.span(1))
	}
	press(m, 's', "s")
	if m.order != ([panelCount]int{1, 7, 3, 6, 2, 5, 4, 0, 8, 10, 9}) {
		t.Fatal("layout sort did not order titles")
	}
	if m.span(1) != (Span{2, 2}) {
		t.Fatal("sorting detached size from panel identity")
	}
	press(m, tea.KeyEscape, "")
	if m.order != order || m.spans != spans {
		t.Fatal("cancel did not undo layout changes")
	}
	shifted(m, tea.KeyRight)
	shifted(m, tea.KeyDown)
	checkPacked(t, m)
	press(m, '0', "0")
	if m.order != order || m.spans != spans {
		t.Fatal("reset did not restore the default grid")
	}
}

func TestMouseCardResizeSnapsAndCancelsOnEveryEdge(t *testing.T) {
	for _, edge := range []string{"left", "right", "top", "bottom", "corner"} {
		t.Run(edge, func(t *testing.T) {
			m := testModel()
			m.kinds[0] = 1 // Use a panel that permits vertical growth.
			m.SetSize(120, 30)
			m.spans[0] = Span{2, 2}
			r := m.cardRect(0)
			base := Grid(m.width, m.height)
			x, y, dx, dy := r.X+r.W-1, r.Y+r.H/2, -base.ColumnStep, 0
			want := Span{1, 2}
			switch edge {
			case "left":
				x = r.X
				dx = base.ColumnStep
			case "top":
				x = r.X + r.W/2
				y = r.Y
				dx = 0
				dy = base.RowStep
				want = Span{2, 1}
			case "bottom":
				x = r.X + r.W/2
				y = r.Y + r.H - 1
				dx = 0
				dy = -base.RowStep
				want = Span{2, 1}
			case "corner":
				y = r.Y + r.H - 1
				dy = -base.RowStep
				want = Span{1, 1}
			}
			m.click(mouse(x, y))
			if m.drag == nil || m.drag.Kind != "card-resize" {
				t.Fatal("edge opened a panel instead of resizing")
			}
			m.motion(tea.MouseMotionMsg{X: x + dx/4, Y: y + dy/4, Button: tea.MouseLeft})
			if m.span(0) != (Span{2, 2}) {
				t.Fatal("sub-grid motion resized a card")
			}
			m.motion(tea.MouseMotionMsg{X: x + dx, Y: y + dy, Button: tea.MouseLeft})
			if m.span(0) != (Span{2, 2}) || !m.drag.Valid || m.drag.PreviewSpan != want {
				t.Fatalf("resize preview got %+v want %+v; committed %+v", m.drag.PreviewSpan, want, m.span(0))
			}
			m.motion(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
			if m.span(0) != (Span{2, 2}) {
				t.Fatal("returning pointer did not restore span")
			}
			m.motion(tea.MouseMotionMsg{X: x + dx, Y: y + dy, Button: tea.MouseLeft})
			press(m, tea.KeyEscape, "")
			if m.span(0) != (Span{2, 2}) || m.drag != nil {
				t.Fatal("Esc did not restore original span")
			}
			checkPacked(t, m)
		})
	}
}

func TestResizedCardKeepsPopoutAndReorderHitTargets(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.resizeCard(1, 1, 0)
	a := m.cardPopout(1)
	m.click(mouse(a.Rect.X+1, a.Rect.Y))
	if m.activeWindow() < 0 || m.selected != 1 {
		t.Fatal("resized pop-out hit target missed")
	}
	press(m, tea.KeyEscape, "")
	from, to := m.cardRect(1), m.cardRect(3)
	m.click(mouse(from.X+8, from.Y+1))
	m.motion(tea.MouseMotionMsg{X: to.X + 8, Y: to.Y + 2, Button: tea.MouseLeft})
	m.release(tea.MouseReleaseMsg{X: to.X + 8, Y: to.Y + 2, Button: tea.MouseLeft})
	if m.slot(1) != 3 || m.span(1) != (Span{2, 1}) {
		t.Fatal("title drop lost the resized panel")
	}
	checkPacked(t, m)
}

func TestReadableNamesAndAdaptiveProcessData(t *testing.T) {
	for raw, want := range map[string]string{"/usr/bin/firefox": "firefox", `C:\Program Files\Editor\code.exe`: "code.exe", "/Volumes/Archive": "Archive", "WindowServer": "WindowServer", "kworker/0:1": "kworker/0:1"} {
		if shortName(raw) != want {
			t.Fatalf("%q rendered as %q", raw, shortName(raw))
		}
	}
	m := testModel()
	m.SetSize(120, 30)
	m.snapshot.Processes[0].Name = "/usr/bin/firefox"
	small := ansi.Strip(strings.Join(m.cardLines(4, 34, 10), "\n"))
	if strings.Contains(small, "/usr/bin") || !strings.Contains(small, "CPU %") || !strings.Contains(small, "MEMORY") || !strings.Contains(small, "terminal") {
		t.Fatal("overview lost names, labeled metrics or usable rows")
	}
	for i := 0; i < 20; i++ {
		m.snapshot.Processes = append(m.snapshot.Processes, metrics.Process{PID: int32(5000 + i), Name: "worker", CPUReady: true})
	}
	large := m.cardLines(4, 74, 25)
	if len(large) <= 10 {
		t.Fatal("larger process panel did not show more data")
	}
	disks := ansi.Strip(strings.Join(m.cardLines(3, 34, 10), "\n"))
	if !strings.Contains(disks, "System") || !strings.Contains(disks, "Archive") || !strings.Contains(disks, "92.3%") || strings.Contains(disks, "LOW SPACE") || strings.Contains(disks, "/Volumes/") {
		t.Fatal("disk overview lost meters or retained noisy labels")
	}
}

func TestArrowNavigationAroundSpanningPanel(t *testing.T) {
	m := testModel()
	m.kinds[0] = 1
	m.SetSize(120, 30)
	m.spans[0] = Span{2, 2}
	m.selected = 1
	press(m, tea.KeyDown, "")
	if m.selected != 3 {
		t.Fatal("Down ignored the neighbor beside a tall panel")
	}
	press(m, tea.KeyLeft, "")
	if m.selected != 0 {
		t.Fatal("Left did not enter the spanning panel")
	}
	press(m, tea.KeyDown, "")
	if m.selected != 0 {
		t.Fatal("Down navigated to a hidden panel below the viewport")
	}
	press(m, tea.KeyTab, "")
	if m.selected != 1 {
		t.Fatal("Tab lost reading order in a packed grid")
	}
}
