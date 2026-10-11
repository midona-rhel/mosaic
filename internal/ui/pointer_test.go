package ui

import (
	byteops "bytes"
	"io"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

func TestTerminalEffectsOptOutRetainsMouseActions(t *testing.T) {
	m := testModel()
	m.opts.NoTerminalEffects = true
	v := m.View()
	if v.BackgroundColor != nil || v.WindowTitle != "" || v.MouseMode != tea.MouseModeAllMotion {
		t.Fatal("effects opt-out changed mouse capture or retained host overrides")
	}
	p := WithMousePointer(m).(*pointerModel)
	a := m.cardPopout(0).Rect
	m.motion(tea.MouseMotionMsg{X: a.X, Y: a.Y})
	if p.syncPointer() != nil {
		t.Fatal("effects opt-out emitted a pointer override")
	}
	m.click(mouse(a.X, a.Y))
	if len(m.windows) != 1 {
		t.Fatal("effects opt-out disabled the pop-out action")
	}
}

func TestPointerMatchesControlsAndTopmostWindow(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	check := func(x, y int, want string) {
		t.Helper()
		m.motion(tea.MouseMotionMsg{X: x, Y: y})
		if got := m.pointerShape(); got != want {
			t.Fatalf("at %d,%d: got %s, want %s", x, y, got, want)
		}
	}
	r := m.cardRect(0)
	check(r.X+3, r.Y+4, "default")
	button := m.cardPopout(0).Rect
	check(button.X, button.Y, "pointer")
	check(r.X, r.Y, "default")
	check(r.X+r.W-1, r.Y, "default")
	check(r.X, r.Y+3, "default")
	check(r.X+3, r.Y+r.H-1, "default")
	check(r.X+3, r.Y+1, "grab")
	m.click(mouse(r.X+3, r.Y+1))
	if m.pointerShape() != "grabbing" {
		t.Fatal("dragging title did not change pointer")
	}
	m.drag = nil
	m.floatPanel(1)
	m.windows[0].Bounds = Rect{button.X - 2, button.Y - 2, 30, 12}
	check(button.X, button.Y, "default") // Covered dashboard button.
	a := m.windowActions(m.windows[0])[0].Rect
	check(a.X, a.Y, "pointer")
	m.help = true
	check(a.X, a.Y, "default") // Modal blocks window actions.
	m.help = false
	as, _ := m.actionBar()
	check(as[0].Rect.X, as[0].Rect.Y, "pointer")
}

func TestPointerOutputDeduplicatedAndOptOut(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ITERM_SESSION_ID", "")
	m := testModel()
	p := WithMousePointer(m).(*pointerModel)
	p.shape = ""
	cmd := p.syncPointer()
	if cmd == nil || cmd().(tea.RawMsg).Msg != "\x1b]22;default\x07" {
		t.Fatal("missing initial arrow request")
	}
	if p.syncPointer() != nil {
		t.Fatal("unchanged pointer emitted repeated output")
	}
	m.opts.NoMouse = true
	m.mouseX, m.mouseY = 1, m.height-1
	if p.syncPointer() != nil {
		t.Fatal("--no-mouse changed the pointer")
	}
}

func TestPointerScreenLifecycle(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ITERM_SESSION_ID", "")
	file, ok := PointerOutput(os.Stdout).(term.File)
	if !ok || file.Fd() != os.Stdout.Fd() {
		t.Fatal("pointer writer lost terminal identity")
	}
	var output byteops.Buffer
	w := PointerOutput(&output)
	in := "before\x1b[?1049hcontent\x1b[?1049lafter"
	n, err := io.WriteString(w, in)
	want := "before\x1b[?1049h\x1b]22;default\x07content\x1b]22;\x07\x1b[?1049lafter"
	if err != nil || n != len(in) || output.String() != want {
		t.Fatalf("incorrect pointer lifecycle: %q, %d, %v", output.String(), n, err)
	}
	output.Reset()
	_, _ = io.WriteString(w, "ordinary output")
	if strings.Contains(output.String(), "22;") {
		t.Fatal("changed pointer without entering alternate screen")
	}
}

func TestITermLegacyPointersAndCursorGuide(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	for shape, want := range map[string]string{"default": "arrow", "pointer": "hand2", "ew-resize": "sb_h_double_arrow", "ns-resize": "sb_v_double_arrow", "grab": "fleur", "nwse-resize": "arrow", "nesw-resize": "arrow"} {
		if pointerSequence(shape) != "\x1b]22;"+want+"\x07" {
			t.Fatalf("wrong iTerm name for %s", shape)
		}
	}
	var out byteops.Buffer
	w := PointerOutput(&out)
	_, _ = io.WriteString(w, "\x1b[?1049h")
	if out.String() != "\x1b[?1049h\x1b]22;arrow\x07\x1b]1337;HighlightCursorLine=0\x07" {
		t.Fatalf("missing iTerm setup: %q", out.String())
	}
}

func TestHoverCacheInvalidatesOnActionsResizeAndData(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	p := WithMousePointer(m).(*pointerModel)
	p.Update(tea.MouseMotionMsg{X: 6, Y: 8})
	first := p.View().Content
	p.Update(tea.MouseMotionMsg{X: 7, Y: 8})
	if !p.valid || p.View().Content != first {
		t.Fatal("plain hover invalidated the frame")
	}
	a := m.cardPopout(0).Rect
	p.Update(tea.MouseMotionMsg{X: a.X, Y: a.Y})
	if p.valid {
		t.Fatal("button hover failed to invalidate frame")
	}
	p.View()
	p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if p.valid {
		t.Fatal("resize reused a stale frame")
	}
	p.View()
	p.Update(sampleMsg(m.snapshot))
	if p.valid {
		t.Fatal("telemetry update reused a stale frame")
	}
}
