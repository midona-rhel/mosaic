package ui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/midona-rhel/mosaic/internal/metrics"
	"testing"
)

func BenchmarkContentHover(b *testing.B) {
	m := testModel()
	m.SetSize(120, 30)
	for i := 0; i < 1200; i++ {
		m.snapshot.Processes = append(m.snapshot.Processes, metrics.Process{PID: int32(10000 + i), Name: fmt.Sprintf("worker-%d", i), CPU: float64(i % 100), CPUReady: true})
	}
	p := WithMousePointer(m)
	p.View()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Update(tea.MouseMotionMsg{X: 5 + i%10, Y: 8})
		p.View()
	}
}

func BenchmarkGridResizeMotion(b *testing.B) {
	for _, cached := range []bool{false, true} {
		name := "repaint every event"
		if cached {
			name = "reuse unchanged proposal"
		}
		b.Run(name, func(b *testing.B) {
			m := testModel()
			m.SetSize(120, 30)
			for i := 0; i < 1200; i++ {
				m.snapshot.Processes = append(m.snapshot.Processes, metrics.Process{PID: int32(10000 + i), Name: fmt.Sprintf("worker-%d", i), CPU: float64(i % 100), CPUReady: true})
			}
			p := WithMousePointer(m).(*pointerModel)
			r := m.cardRect(1)
			p.Update(mouse(r.X+r.W-1, r.Y+4))
			p.Update(tea.MouseMotionMsg{X: r.X + r.W, Y: r.Y + 4, Button: tea.MouseLeft})
			p.View()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p.Update(tea.MouseMotionMsg{X: r.X + r.W + i%8, Y: r.Y + 4, Button: tea.MouseLeft})
				if !cached {
					p.valid = false
				}
				p.View()
			}
		})
	}
}
