package ui

import (
	"strings"

	"github.com/midona-rhel/mosaic/internal/metrics"
)

const historyLimit = 120

func history(values []float64, v float64) []float64 {
	values = append(values, v)
	if len(values) > historyLimit {
		values = values[len(values)-historyLimit:]
	}
	return values
}
func (m *Model) accept(s metrics.Snapshot) {
	if m.gpuHistory == nil {
		m.gpuHistory = map[string][]float64{}
	}
	if !s.GPU.At.Equal(m.snapshot.GPU.At) {
		keys := map[string]bool{}
		for _, g := range s.GPU.Devices {
			key := g.Source + "/" + g.ID
			keys[key] = true
			if g.Utilization != nil {
				m.gpuHistory[key] = history(m.gpuHistory[key], *g.Utilization)
			}
		}
		for key := range m.gpuHistory {
			if !keys[key] {
				delete(m.gpuHistory, key)
			}
		}
	}
	if s.DiskReady {
		if m.diskHistory == nil {
			m.diskHistory = map[string][2][]float64{}
		}
		next := map[string][2][]float64{}
		for _, d := range s.Disks {
			v := m.diskHistory[d.Name]
			next[d.Name] = [2][]float64{history(v[0], d.Read), history(v[1], d.Write)}
		}
		m.diskHistory = next
	}
	m.snapshot = s
	if s.CPUReady {
		m.cpuHistory = history(m.cpuHistory, average(s.CPU))
	}
	if s.Memory != nil {
		m.memHistory = history(m.memHistory, s.Memory.UsedPercent)
	}
	if s.RatesReady {
		down, _ := traffic(s)
		m.netHistory = history(m.netHistory, down)
	}
	if strings.HasPrefix(m.status, "Collecting") {
		m.status = ""
	}
	m.ensureProcess()
	m.clampOffsets()
}
func average(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}
func traffic(s metrics.Snapshot) (float64, float64) {
	down, up := 0.0, 0.0
	for _, n := range s.Network {
		if !n.Ready {
			continue
		}
		if s.Loopback[n.Name] || n.Name == "lo" || n.Name == "lo0" || strings.HasPrefix(strings.ToLower(n.Name), "loopback") {
			continue
		}
		down += n.Down
		up += n.Up
	}
	return down, up
}
