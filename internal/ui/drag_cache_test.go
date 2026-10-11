package ui

import (
	tea "charm.land/bubbletea/v2"
	"testing"
	"time"
)

// Compare cached composition with a fresh render at every drag position,
// including overlap, resizes and live data updates during the gesture.
func TestDragCacheMatchesFreshRender(t *testing.T) {
	for _, kind := range []string{"move", "resize", "card", "card-resize"} {
		t.Run(kind, func(t *testing.T) {
			m := testModel()
			m.SetSize(160, 45)
			r := m.cardRect(1)
			if kind == "move" || kind == "resize" {
				m.floatPanel(1)
				r = m.windows[0].Bounds
			}
			x, y := r.X+3, r.Y+1
			if kind == "resize" || kind == "card-resize" {
				x, y = r.X+r.W-1, r.Y+r.H-1
			}
			m.Update(mouse(x, y))
			if m.drag == nil || m.drag.Kind != kind {
				t.Fatalf("gesture: %+v", m.drag)
			}
			for i := 0; i < 12; i++ {
				m.Update(tea.MouseMotionMsg{X: x + i*2, Y: y + i/3, Button: tea.MouseLeft})
				got := m.render()
				fresh := *m
				fresh.dragPaint = nil
				fresh.dragCells = nil
				want := fresh.render()
				if got != want {
					t.Fatalf("cached frame differs at position %d", i)
				}
				if i == 6 {
					s := m.snapshot
					s.CPU = []float64{92, 83}
					m.SetSnapshot(s)
					if m.dragPaint != nil || m.dragCells != nil {
						t.Fatal("sample retained stale cache")
					}
				}
			}
			m.Update(tea.MouseReleaseMsg{X: x + 22, Y: y + 3, Button: tea.MouseLeft})
			if m.dragPaint != nil || m.dragCells != nil {
				t.Fatal("release retained drag cache")
			}
		})
	}
}

func TestDragPaintsImmediatelyWhenFrameIsDue(t *testing.T) {
	m := testModel()
	m.SetSize(160, 45)
	m.floatPanel(1)
	p := WithMousePointer(m).(*pointerModel)
	r := m.windows[0].Bounds
	p.Update(mouse(r.X+3, r.Y+1))
	p.View()
	p.lastFrame = time.Now().Add(-time.Second)
	p.Update(tea.MouseMotionMsg{X: r.X + 8, Y: r.Y + 1, Button: tea.MouseLeft})
	if p.valid || p.framePending {
		t.Fatal("due frame must render immediately without another timer")
	}
}
