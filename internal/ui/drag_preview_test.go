package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDragPreviewCommitsOnlyOnRelease(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	saved := m.Workspace()
	from, to := m.cardRect(4), m.cardRect(0)
	m.click(mouse(from.X+5, from.Y+1))
	m.motion(tea.MouseMotionMsg{X: to.X + 5, Y: to.Y + 2, Button: tea.MouseLeft})
	if m.Workspace() != saved || !m.drag.Valid || m.drag.Preview != to {
		t.Fatal("move changed workspace before drop or wrong outline")
	}
	text := ansi.Strip(m.render())
	assertContinuousPreview(t, m)
	if strings.Contains(text, "Release to place") {
		t.Fatal("instructional drag hint still appears above the footer")
	}
	preview := m.drag.Preview
	m.release(tea.MouseReleaseMsg{X: to.X + 5, Y: to.Y + 2, Button: tea.MouseLeft})
	if m.order[0] != 4 || m.cardRect(4) != preview || m.spans != saved.Spans {
		t.Fatal("drop did not match preview or changed spans")
	}
	for _, p := range m.order {
		if m.cardRect(p).W > 0 && m.cardRect(p).H != from.H {
			t.Fatal("reorder changed a card's height")
		}
	}
}

func TestGrowingBottomCardUpDisplacesEarlierCard(t *testing.T) {
	m := testModel()
	m.SetSize(55, 20) // two columns, two usable rows
	m.spans[2] = Span{2, 1}
	r := m.cardRect(2)
	base := m.layout()
	before := m.Workspace()
	m.click(mouse(r.X+4, r.Y))
	m.motion(tea.MouseMotionMsg{X: r.X + 4, Y: r.Y - base.RowStep, Button: tea.MouseLeft})
	if m.Workspace() != before || !m.drag.Valid || m.drag.PreviewOrder[0] != 2 || m.drag.PreviewSpan != (Span{2, 2}) {
		t.Fatalf("bad upward preview: %+v", m.drag)
	}
	if !strings.Contains(m.dragHint(), "Will hide:") || !strings.Contains(m.dragHint(), "System") {
		t.Fatal("preview did not show displaced panel")
	}
	preview := m.drag.Preview
	m.release(tea.MouseReleaseMsg{X: r.X + 4, Y: r.Y - base.RowStep, Button: tea.MouseLeft})
	if m.selected != 2 || m.cardRect(2) != preview || m.cardRect(0).W != 0 {
		t.Fatal("upward growth hid the selected panel instead of the displaced one")
	}
	for p := range m.spans {
		if p != 2 && m.spans[p] != before.Spans[p] {
			t.Fatal("growth changed another panel's span")
		}
	}
}

func TestReorderKeepsGeometryWhenPackingEfficiencyChanges(t *testing.T) {
	m := testModel()
	m.SetSize(120, 56)
	m.spans[0], m.spans[1] = Span{3, 1}, Span{2, 2}
	before := m.layout()
	spans := m.spans
	m.moveCard(1, 0)
	after := m.layout()
	if before.Rows != after.Rows || before.CardHeight != after.CardHeight || before.RowStep != after.RowStep || m.spans != spans {
		t.Fatal("reorder changed grid units")
	}
}

func TestWindowDragShowsLiveBodyAndCanCancel(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.floatPanel(1)
	r := m.windows[0].Bounds
	m.click(mouse(r.X+5, r.Y+1))
	m.motion(tea.MouseMotionMsg{X: r.X + 10, Y: r.Y + 2, Button: tea.MouseLeft})
	if m.windows[0].Bounds != r || !m.drag.Valid {
		t.Fatal("window bounds changed during preview")
	}
	text := ansi.Strip(m.render())
	if !strings.Contains(strings.Split(text, "\n")[m.drag.Preview.Y+1], ":: CPU") || !strings.Contains(text, "Total utilization") {
		t.Fatal("drag did not render the window contents at the new position")
	}
	canvas := lipgloss.NewCanvas(m.width, m.height)
	m.paintDragPreview(canvas)
	if c := canvas.CellAt(m.drag.Preview.X, m.drag.Preview.Y); c != nil && c.Content != "" && c.Content != " " {
		t.Fatal("floating drag still draws a white outline")
	}
	press(m, tea.KeyEscape, "")
	if m.windows[0].Bounds != r || m.drag != nil {
		t.Fatal("cancel changed original window")
	}
}

func TestHoverBorderAndCacheFollowTopmostSurface(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	p := WithMousePointer(m).(*pointerModel)
	r := m.cardRect(1)
	p.Update(tea.MouseMotionMsg{X: r.X + 3, Y: r.Y + 3})
	if m.hoveredSurface() != r || !strings.Contains(p.View().Content, "119;153;191") {
		t.Fatal("clickable panel lacks distinct hover border")
	}
	s := m.cardRect(2)
	p.Update(tea.MouseMotionMsg{X: s.X + 3, Y: s.Y + 3})
	if p.valid || m.hoveredSurface() != s {
		t.Fatal("cached hover remained on previous card")
	}
	m.floatPanel(3)
	m.windows[0].Bounds = Rect{r.X + 1, r.Y + 1, 40, 20}
	m.motion(tea.MouseMotionMsg{X: r.X + 3, Y: r.Y + 3})
	if m.hoveredSurface() == r {
		t.Fatal("hover leaked through window")
	}
}

func TestPanelOrganizerRestoresHiddenPanelWithoutLosingSize(t *testing.T) {
	m := testModel()
	m.SetSize(55, 24)
	m.spans[0] = Span{2, 2}
	before := m.spans
	press(m, 'p', "p")
	press(m, 'o', "o")
	if !m.organizing {
		t.Fatal("panel organizer did not open")
	}
	text := ansi.Strip(m.render())
	if !strings.Contains(text, "Hidden") || !strings.Contains(text, "PRIORITY ORDER") {
		t.Fatal("hidden panels not discoverable")
	}
	press(m, '7', "7")
	press(m, tea.KeyEnter, "")
	if m.organizing || m.order[0] != 6 || m.cardRect(6).W == 0 || m.spans != before {
		t.Fatal("restoring hidden panel lost size or priority")
	}
	press(m, 'o', "o")
	press(m, '1', "1")
	press(m, 'f', "f")
	if !m.expanded || m.selected != 0 || m.reordering {
		t.Fatal("hidden panel could not be opened directly")
	}
	for _, size := range [][2]int{{24, 10}, {55, 24}, {120, 30}} {
		m.SetSize(size[0], size[1])
		m.organizing = true
		for i := range panelCount {
			m.organizerIndex = i
			assertFits(t, m)
		}
	}
}

// Check every outline cell, including the former title gap, against the actual
// window border. Nothing in the transparent interior should be painted.
func assertContinuousPreview(t *testing.T, m *Model) {
	t.Helper()
	canvas := lipgloss.NewCanvas(m.width, m.height)
	m.paintDragPreview(canvas)
	r := m.drag.Preview
	b := lipgloss.RoundedBorder()
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			want := ""
			switch {
			case y == r.Y && x == r.X:
				want = b.TopLeft
			case y == r.Y && x == r.X+r.W-1:
				want = b.TopRight
			case y == r.Y+r.H-1 && x == r.X:
				want = b.BottomLeft
			case y == r.Y+r.H-1 && x == r.X+r.W-1:
				want = b.BottomRight
			case y == r.Y:
				want = b.Top
			case y == r.Y+r.H-1:
				want = b.Bottom
			case x == r.X:
				want = b.Left
			case x == r.X+r.W-1:
				want = b.Right
			}
			cell := canvas.CellAt(x, y)
			if want == "" {
				if cell != nil && cell.Content != " " && cell.Content != "" {
					t.Fatalf("preview painted interior at %d,%d", x, y)
				}
			} else if cell == nil || cell.Content != want || cell.Style.Fg != ink {
				t.Fatalf("preview border at %d,%d: got %+v, want white %q", x, y, cell, want)
			}
		}
	}
}
