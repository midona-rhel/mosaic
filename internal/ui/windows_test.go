package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func mouse(x, y int) tea.MouseClickMsg { return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft} }
func TestPopoutMovesResizesAndDocks(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	a := m.cardPopout(1)
	m.click(mouse(a.Rect.X+1, a.Rect.Y))
	if m.expanded || len(m.windows) != 1 || m.focusWindow != 1 {
		t.Fatal("pop-out button did not detach CPU")
	}
	i := m.activeWindow()
	w := m.windows[i].Bounds
	assertFits(t, m)
	m.click(mouse(w.X+4, w.Y+1))
	m.motion(tea.MouseMotionMsg{X: w.X + 9, Y: w.Y + 3, Button: tea.MouseLeft})
	m.release(tea.MouseReleaseMsg{})
	if m.windows[i].Bounds == w {
		t.Fatal("window did not move")
	}
	w = m.windows[i].Bounds
	m.click(mouse(w.X+w.W-1, w.Y+w.H-1))
	m.motion(tea.MouseMotionMsg{X: w.X + w.W - 6, Y: w.Y + w.H - 4, Button: tea.MouseLeft})
	m.release(tea.MouseReleaseMsg{})
	assertFits(t, m)
	if m.windows[i].Bounds.W == w.W && m.windows[i].Bounds.H == w.H {
		t.Fatal("window did not resize")
	}
	press(m, 'm', "m")
	if !m.expanded || m.selected != 1 || len(m.windows) != 0 {
		t.Fatal("maximize failed")
	}
	press(m, 'f', "f")
	press(m, tea.KeyEscape, "")
	if len(m.windows) != 0 || m.expanded {
		t.Fatal("dock did not restore grid")
	}
}
func TestTitleClickWithoutDragSelects(t *testing.T) {
	m := testModel()
	r := Grid(80, 24).Cards[2]
	m.click(mouse(r.X+5, r.Y+1))
	m.release(tea.MouseReleaseMsg{})
	if m.expanded || m.selected != 2 || len(m.windows) != 0 {
		t.Fatal("simple title click failed")
	}
}
func TestClickTargetsTopWindowAndModalBlocksUnderlying(t *testing.T) {
	m := testModel()
	m.SetSize(120, 35)
	m.floatPanel(1)
	m.floatPanel(3)
	for i := range m.windows {
		m.windows[i].Bounds = Rect{15, 5, 64, 24}
	}
	m.click(mouse(25, 10))
	if m.focusWindow != 3 {
		t.Fatal("click passed through top disk window")
	}
	m.floatPanel(4)
	cmd := m.openProcess()
	m.Update(cmd())
	m.confirm = true
	r := m.windows[m.windowIndex(4)].Bounds
	m.click(mouse(r.X+3, r.Y+1))
	if m.drag != nil {
		t.Fatal("confirmation allowed dragging underlying window")
	}
	before := m.focusWindow
	m.click(mouse(2, 4))
	if m.focusWindow != before || !m.confirm {
		t.Fatal("confirmation click passed through")
	}
}
func TestClickableActionsAndInspectorDrag(t *testing.T) {
	m := testModel()
	as, _ := m.actionBar()
	for _, a := range as {
		if a.Key.Code == ' ' {
			m.click(mouse(a.Rect.X, a.Rect.Y))
		}
	}
	if !m.paused {
		t.Fatal("pause button did not pause")
	}
	press(m, '5', "5")
	cmd := m.openProcess()
	m.Update(cmd())
	r := m.inspectorRect()
	m.click(mouse(r.X+5, r.Y+1))
	m.motion(tea.MouseMotionMsg{X: r.X + 9, Y: r.Y + 3, Button: tea.MouseLeft})
	m.release(tea.MouseReleaseMsg{})
	if m.inspectorRect() == r {
		t.Fatal("inspector did not move")
	}
	as, _ = m.actionBar()
	for _, a := range as {
		if a.Key.Code == 'x' {
			m.click(mouse(a.Rect.X, a.Rect.Y))
		}
	}
	if !m.confirm {
		t.Fatal("terminate button bypassed or failed confirmation")
	}
	as, _ = m.actionBar()
	for _, a := range as {
		if a.Key.Code == tea.KeyEscape {
			m.click(mouse(a.Rect.X, a.Rect.Y))
		}
	}
	if m.confirm || m.target == nil {
		t.Fatal("cancel should only close confirmation")
	}
}
func TestFloatingViewsClampOnResize(t *testing.T) {
	for _, w := range []int{24, 40, 80, 120} {
		for _, h := range []int{10, 12, 24, 40} {
			m := testModel()
			m.SetSize(160, 45)
			for i := range panelCount {
				m.offset = 0
				m.floatPanel(i)
			}
			m.SetSize(w, h)
			for _, win := range m.windows {
				r := win.Bounds
				if r.X < 1 || r.Y < 2 || r.X+r.W > w-1 || r.Y+r.H > h-2 {
					t.Fatalf("window escaped %dx%d: %+v", w, h, r)
				}
			}
			assertFits(t, m)
			cmd := m.openProcess()
			if cmd != nil {
				m.Update(cmd())
				assertFits(t, m)
			}
		}
	}
}
func TestPaletteAndGradient(t *testing.T) {
	if usageHex(0) != "#538FFF" || usageHex(60) != "#B45CFF" || usageHex(100) != "#FF456D" {
		t.Fatal("palette stops changed")
	}
	if ansi.StringWidth(bar(95, 40)) != 40 {
		t.Fatal("gradient changed cell width")
	}
	if strings.Count(bar(95, 40), "38;2;") < 3 {
		t.Fatal("high usage lost red transition")
	}
	m := testModel()
	m.selected = 3
	details := ansi.Strip(strings.Join(m.detailLines(80), "\n"))
	if !strings.Contains(details, "92.3%") || strings.Contains(details, "LOW SPACE") {
		t.Fatal("disk usage should remain visible without the warning banner")
	}
}
func TestSingleFloatingWindowTabKeepsSelectionAndEscapeCancelsDrag(t *testing.T) {
	m := testModel()
	m.floatPanel(1)
	press(m, tea.KeyTab, "")
	if m.selected != 1 || m.focusWindow != 1 {
		t.Fatal("single-window Tab changed the wrong panel")
	}
	r := m.windows[0].Bounds
	m.click(mouse(r.X+5, r.Y+1))
	m.motion(tea.MouseMotionMsg{X: r.X + 10, Y: r.Y + 3, Button: tea.MouseLeft})
	press(m, tea.KeyEscape, "")
	if m.drag != nil || m.windows[0].Bounds != r {
		t.Fatal("Esc did not restore drag origin")
	}
}
func TestWheelTargetsWindowUnderPointerAndHelpScrolls(t *testing.T) {
	m := testModel()
	m.SetSize(120, 35)
	m.floatPanel(1)
	m.floatPanel(3)
	m.windows[0].Bounds = Rect{1, 2, 45, 20}
	m.windows[1].Bounds = Rect{60, 2, 50, 20}
	m.Update(tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelDown})
	if m.focusWindow != 1 {
		t.Fatal("wheel scrolled the wrong floating window")
	}
	press(m, '?', "?")
	m.SetSize(40, 12)
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.helpOffset == 0 {
		t.Fatal("help did not scroll with mouse wheel")
	}
}
func TestWindowTitleButtonsAndNoMouse(t *testing.T) {
	m := testModel()
	m.floatPanel(1)
	r := m.windows[0].Bounds
	m.click(mouse(r.X+r.W-8, r.Y+1))
	if !m.expanded {
		t.Fatal("maximize title button failed")
	}
	press(m, 'f', "f")
	r = m.windows[0].Bounds
	m.click(mouse(r.X+r.W-4, r.Y+1))
	if len(m.windows) != 0 {
		t.Fatal("dock title button failed")
	}
	m.opts.NoMouse = true
	m.click(mouse(5, 3))
	m.motion(tea.MouseMotionMsg{X: 10, Y: 10, Button: tea.MouseLeft})
	if m.drag != nil || m.expanded {
		t.Fatal("--no-mouse captured input")
	}
}

func TestEveryWindowEdgeAndCornerResizesWithOppositeEdgeAnchored(t *testing.T) {
	origin := Rect{30, 8, 48, 16}
	cases := []struct {
		name  string
		x, y  int
		edges resizeEdge
	}{
		{"left", 30, 15, resizeLeft}, {"right", 77, 15, resizeRight},
		{"top", 50, 8, resizeTop}, {"bottom", 50, 23, resizeBottom},
		{"top-left", 30, 8, resizeLeft | resizeTop},
		{"top-right", 77, 8, resizeRight | resizeTop},
		{"bottom-left", 30, 23, resizeLeft | resizeBottom},
		{"bottom-right", 77, 23, resizeRight | resizeBottom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, inspector := range []bool{false, true} {
				m := testModel()
				m.SetSize(120, 40)
				m.floatPanel(1)
				m.windows[0].Bounds = origin
				if inspector {
					m.Update(m.openProcess()())
					r := origin
					m.inspectorBounds = &r
				}
				m.click(mouse(tc.x, tc.y))
				if m.drag == nil || m.drag.Edges != tc.edges {
					t.Fatalf("edge didn't start resize: %+v", m.drag)
				}
				m.motion(tea.MouseMotionMsg{X: tc.x + 5, Y: tc.y + 3, Button: tea.MouseLeft})
				r := m.windows[0].Bounds
				if inspector {
					r = *m.inspectorBounds
					if m.inspectorRect() != m.drag.Preview {
						t.Fatal("inspector does not follow the pointer")
					}
				}
				if r != origin {
					t.Fatal("committed bounds changed before release")
				}
				r = m.drag.Preview
				want := origin
				if tc.edges&resizeLeft != 0 {
					want.X += 5
					want.W -= 5
				}
				if tc.edges&resizeRight != 0 {
					want.W += 5
				}
				if tc.edges&resizeTop != 0 {
					want.Y += 3
					want.H -= 3
				}
				if tc.edges&resizeBottom != 0 {
					want.H += 3
				}
				if r != want {
					t.Fatalf("resize got %+v, want %+v", r, want)
				}
				assertFits(t, m)
				// Moving back to the initial pointer cell must undo the live resize.
				m.motion(tea.MouseMotionMsg{X: tc.x, Y: tc.y, Button: tea.MouseLeft})
				r = m.windows[0].Bounds
				if inspector {
					r = m.inspectorRect()
				}
				if r != origin || m.drag.Preview != origin {
					t.Fatal("returning the pointer left a stale resize")
				}
				m.motion(tea.MouseMotionMsg{X: tc.x + 5, Y: tc.y + 3, Button: tea.MouseLeft})
				// Escape restores both kinds of windows before the mouse is released.
				press(m, tea.KeyEscape, "")
				r = m.windows[0].Bounds
				if inspector {
					r = m.inspectorRect()
				}
				if r != origin || m.drag != nil {
					t.Fatal("cancel did not restore original bounds")
				}
			}
		})
	}
}

func TestResizeClampsMovingEdgesWithoutMovingTheirAnchors(t *testing.T) {
	origin := Rect{30, 8, 48, 16}
	cases := []struct {
		e      resizeEdge
		dx, dy int
		want   Rect
	}{
		{resizeLeft | resizeTop, 1000, 1000, Rect{54, 16, 24, 8}},
		{resizeLeft | resizeTop, -1000, -1000, Rect{1, 2, 77, 22}},
		{resizeRight | resizeBottom, -1000, -1000, Rect{30, 8, 24, 8}},
		{resizeRight | resizeBottom, 1000, 1000, Rect{30, 8, 89, 30}},
	}
	for _, tc := range cases {
		if got := resizeBounds(origin, tc.e, tc.dx, tc.dy, 120, 40); got != tc.want {
			t.Fatalf("clamp got %+v, want %+v", got, tc.want)
		}
	}
	// Side borders beside title buttons must still resize, never dock/maximize.
	m := testModel()
	m.floatPanel(1)
	r := m.windows[0].Bounds
	m.click(mouse(r.X+r.W-1, r.Y+1))
	if len(m.windows) != 1 || m.expanded || m.drag == nil || m.drag.Edges != resizeRight {
		t.Fatal("title-row edge activated a title button")
	}
}

func TestBracketedProgressGlyphsKeepTerminalCellWidths(t *testing.T) {
	for _, fira := range []bool{false, true} {
		for _, width := range []int{0, 1, 2, 3, 12, 40} {
			for _, value := range []float64{-1, 0, 25, 60, 95, 100, 150} {
				b := progressBar(value, width, fira)
				if ansi.StringWidth(b) != width {
					t.Fatalf("meter changed width %d: %q", width, b)
				}
				plain := ansi.Strip(b)
				if width >= 3 {
					if fira {
						chars := []rune(plain)
						if (chars[0] != 0xEE00 && chars[0] != 0xEE03) || (chars[width-1] != 0xEE02 && chars[width-1] != 0xEE05) {
							t.Fatal("Fira Code meter caps missing")
						}
					} else if !strings.HasPrefix(plain, "[") || !strings.HasSuffix(plain, "]") {
						t.Fatal("fallback brackets missing")
					}
				}
			}
		}
	}
	t.Setenv("UNICODE_PROGRESS_BAR", "true")
	if bar(60, 12) != progressBar(60, 12, true) {
		t.Fatal("upstream font opt-in ignored")
	}
	t.Setenv("UNICODE_PROGRESS_BAR", "false")
	if strings.ContainsAny(ansi.Strip(bar(60, 12)), "\uEE00\uEE01\uEE02\uEE03\uEE04\uEE05") {
		t.Fatal("private glyph leaked into fallback")
	}
}

func TestHotkeysStayClickableAndLeftAligned(t *testing.T) {
	m := testModel()
	as, _ := m.actionBar()
	if len(as) < 4 || as[0].Rect.X != 1 || as[0].Shortcut != "enter" {
		t.Fatal("dashboard hotkeys are not visible at the bottom left")
	}
	for _, a := range as {
		if a.Key.Code == '?' {
			m.click(mouse(a.Rect.X, a.Rect.Y))
		}
	}
	if !m.help {
		t.Fatal("help hit target disagrees with rendered position")
	}
	press(m, tea.KeyEscape, "")
	m.Update(m.openProcess()())
	m.confirm = true
	m.SetSize(24, 12)
	as, _ = m.actionBar()
	if len(as) != 2 {
		t.Fatal("narrow confirmation hid cancel or terminate")
	}
	m.click(mouse(as[0].Rect.X, as[0].Rect.Y))
	if m.confirm || m.target == nil {
		t.Fatal("narrow cancel did not work")
	}
}
