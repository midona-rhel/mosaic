package ui

import (
	byteops "bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

// WithMousePointer adds OSC 22 pointer feedback to the interactive program.
// Snapshot rendering and --no-mouse leave the terminal's pointer alone.
func WithMousePointer(model *Model) tea.Model {
	return &pointerModel{Model: model, shape: "default"}
}

// PointerOutput brackets pointer overrides with Bubble Tea's screen lifecycle.
// Some hosts keep separate pointer settings for the main and alternate screens:
// set the arrow AFTER entering and reset BEFORE leaving, including on signals.
// Bubble Tea emits each screen transition as a complete sequence in one Write.
func PointerOutput(w io.Writer) io.Writer {
	if file, ok := w.(term.File); ok {
		return pointerFile{file}
	}
	return pointerOutput{w}
}

// Preserve terminal detection, size queries and Windows VT initialization.
type pointerFile struct{ term.File }

func (w pointerFile) Write(p []byte) (int, error) { return pointerOutput{w.File}.Write(p) }

type pointerOutput struct{ io.Writer }

func (w pointerOutput) Write(p []byte) (int, error) {
	enter, leave := []byte(ansi.SetModeAltScreenSaveCursor), []byte(ansi.ResetModeAltScreenSaveCursor)
	setup := pointerSequence("default")
	if isITerm() {
		// This changes the current session, not the saved iTerm profile. There
		// is no query for its previous value; do not guess a value on exit.
		setup += ansi.ITerm2("HighlightCursorLine=0")
	}
	out := byteops.ReplaceAll(p, enter, []byte(string(enter)+setup))
	out = byteops.ReplaceAll(out, leave, []byte(ansi.SetPointerShape("")+string(leave)))
	n, err := w.Writer.Write(out)
	if err != nil {
		return 0, err
	}
	if n != len(out) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

type pointerModel struct {
	*Model
	shape           string
	view            tea.View
	valid           bool
	framePending    bool
	frameGeneration uint64
	lastFrame       time.Time
}

type dragFrameMsg uint64

func (p *pointerModel) View() tea.View {
	if !p.valid {
		p.view, p.valid = p.Model.View(), true
		p.lastFrame = time.Now()
	}
	return p.view
}

func (p *pointerModel) hoverVisualKey() string {
	if p.pointerShape() == "default" && p.hoverHint() == "" {
		return "content/" + p.hoverOwnerAt(p.mouseX, p.mouseY)
	}
	return fmt.Sprintf("%d/%d", p.mouseX, p.mouseY)
}

func (p *pointerModel) syncPointer() tea.Cmd {
	if p.opts.NoMouse || p.opts.NoTerminalEffects {
		return nil
	}
	shape := p.pointerShape()
	if shape == p.shape {
		return nil
	}
	p.shape = shape
	return tea.Raw(pointerSequence(shape))
}

func isITerm() bool {
	return strings.EqualFold(os.Getenv("TERM_PROGRAM"), "iTerm.app") || os.Getenv("ITERM_SESSION_ID") != ""
}

func pointerSequence(shape string) string {
	if isITerm() {
		// iTerm's OSC 22 implementation uses X11 names, not CSS names.
		names := map[string]string{"default": "arrow", "pointer": "hand2", "grab": "fleur", "grabbing": "fleur", "ew-resize": "sb_h_double_arrow", "ns-resize": "sb_v_double_arrow", "nwse-resize": "arrow", "nesw-resize": "arrow"}
		if name, ok := names[shape]; ok {
			shape = name
		}
	}
	return ansi.SetPointerShape(shape)
}

func (p *pointerModel) Init() tea.Cmd {
	return p.Model.Init()
}

func (p *pointerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Mouse coordinates are terminal cells, not pixels. Some hosts repeat
	// reports within a cell; ignore them before hit testing or layout work.
	if motion, ok := msg.(tea.MouseMotionMsg); ok && motion.X == p.mouseX && motion.Y == p.mouseY {
		return p, nil
	}

	if frame, ok := msg.(dragFrameMsg); ok {
		if uint64(frame) == p.frameGeneration {
			p.framePending = false
			p.valid = false
		}
		return p, nil
	}
	_, motion := msg.(tea.MouseMotionMsg)
	if !motion {
		p.frameGeneration++
		p.framePending = false
	}
	before := ""
	var dragBefore gesture
	hadDrag := motion && p.drag != nil
	if hadDrag {
		dragBefore = *p.drag
	}
	if motion && p.drag == nil {
		before = p.hoverVisualKey()
	}
	if _, resumed := msg.(tea.ResumeMsg); resumed {
		p.shape = ""
	}
	_, cmd := p.Model.Update(msg)
	// Mouse reports can arrive much faster than the terminal's frame rate.
	// Keep the latest position immediately, but paint once per frame rather
	// than making every intermediate position wait for a complete dashboard.
	if motion && hadDrag && p.drag != nil && p.valid {
		var frame tea.Cmd
		if !p.framePending && dragBefore != *p.drag {
			remaining := time.Until(p.lastFrame.Add(time.Second / 60))
			if remaining <= 0 {
				p.valid = false
			} else {
				p.framePending = true
				generation := p.frameGeneration
				frame = tea.Tick(remaining, func(time.Time) tea.Msg { return dragFrameMsg(generation) })
			}
		}
		return p, tea.Batch(cmd, frame, p.syncPointer())
	}
	// Ordinary content hover has no visible effect. Reuse the last frame
	// instead of rebuilding every graph and sorting process rows per pixel.
	sameProposal := hadDrag && p.drag != nil && dragBefore == *p.drag
	if !sameProposal && (!motion || before == "" || p.drag != nil || before != p.hoverVisualKey()) {
		p.valid = false
	}
	return p, tea.Batch(cmd, p.syncPointer())
}

func resizePointer(e resizeEdge) string {
	switch e {
	case resizeTop | resizeLeft, resizeBottom | resizeRight:
		return "nwse-resize"
	case resizeTop | resizeRight, resizeBottom | resizeLeft:
		return "nesw-resize"
	case resizeTop, resizeBottom:
		return "ns-resize"
	case resizeLeft, resizeRight:
		return "ew-resize"
	}
	return "default"
}

func (m *Model) pointerShape() string {
	x, y := m.mouseX, m.mouseY
	if m.drag != nil {
		if m.drag.Kind == "scroll" {
			return "default"
		}
		if m.drag.Edges != 0 {
			return "default"
		}
		return "grabbing"
	}
	onAction := func(as []action) bool {
		for _, a := range as {
			if !a.Informational && a.Rect.Contains(x, y) {
				return true
			}
		}
		return false
	}
	as, _ := m.actionBar()
	if onAction(as) {
		return "pointer"
	}
	if m.confirm {
		if !m.terminating && onAction(m.confirmationActions()) {
			return "pointer"
		}
		return "default"
	}
	if m.organizing {
		if _, ok := m.organizerRowAt(x, y); ok {
			return "pointer"
		}
		return "default"
	}
	if m.help || m.picking {
		if _, ok := m.scrollbarAt(x, y); ok {
			return "pointer"
		}
		return "default"
	}
	if _, ok := m.scrollbarAt(x, y); ok {
		return "pointer"
	}
	if _, _, ok := m.processSortAt(x, y); ok {
		return "pointer"
	}
	windowShape := func(r Rect, actions []action) string {
		if onAction(actions) {
			return "pointer"
		}
		if e := edgesAt(r, x, y); e != 0 {
			return "default"
		}
		if r.Contains(x, y) && y == r.Y+1 {
			return "grab"
		}
		return "default"
	}
	if m.target != nil {
		return windowShape(m.inspectorRect(), m.inspectorActions())
	}
	if m.expanded {
		if onAction(m.expandedActions()) {
			return "pointer"
		}
		return "default"
	}
	if y < 2 || y >= m.height-2 {
		return "default"
	}
	// The topmost window owns the pointer, even over a dashboard button.
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if w.Bounds.Contains(x, y) {
			return windowShape(w.Bounds, m.windowActions(w))
		}
	}
	for _, panel := range m.order {
		r := m.cardRect(panel)
		if r.Contains(x, y) {
			return windowShape(r, []action{m.cardPopout(panel), m.cardHide(panel)})
		}
	}
	return "default"
}
