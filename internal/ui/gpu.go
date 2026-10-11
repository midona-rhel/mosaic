package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func (m *Model) gpuLines(w int) []string {
	r := m.snapshot.GPU
	lines := []string{}
	add := func(label, value string) { lines = append(lines, detailPair(label, value, w)...) }
	note := func(value string) {
		for _, line := range strings.Split(ansi.Hardwrap(Clean(value), max(1, w), true), "\n") {
			lines = append(lines, dim.Render(line))
		}
	}
	for _, g := range r.Devices {
		lines = append(lines, strong.Render(Clean(g.Name)))
		label := "Utilization"
		if g.UtilizationCounter != "" {
			label = "Driver activity"
		}
		if g.Utilization != nil {
			lines = append(lines, pair(dim.Render(label), metric(fmt.Sprintf("%.1f%%", *g.Utilization), *g.Utilization), w), bar(*g.Utilization, w), "")
		} else {
			add("Utilization", "Unavailable")
		}
		if g.MemoryUsed != nil && g.MemoryTotal != nil && *g.MemoryTotal > 0 {
			add("Dedicated memory", bytes(*g.MemoryUsed)+" / "+bytes(*g.MemoryTotal))
			lines = append(lines, bar(100*float64(*g.MemoryUsed)/float64(*g.MemoryTotal), w), "")
		} else {
			if g.MemoryUsed != nil {
				add("Dedicated allocation", bytes(*g.MemoryUsed))
			}
			if g.MemoryTotal != nil {
				add("Dedicated capacity", bytes(*g.MemoryTotal))
			}
		}
		if g.SharedUsed != nil {
			add("Shared RAM allocation", bytes(*g.SharedUsed))
		}
		if g.MemoryUsed == nil && g.MemoryTotal == nil && g.SharedUsed == nil {
			add("GPU memory", "Unavailable")
		}
		if g.Temperature != nil {
			add("Temperature", fmt.Sprintf("%.1f°C", *g.Temperature))
		} else {
			add("Temperature", "Unavailable")
		}
		if g.Power != nil {
			add("Power", fmt.Sprintf("%.1f W", *g.Power))
		} else {
			add("Power", "Unavailable")
		}
		if values := m.gpuHistory[g.Source+"/"+g.ID]; len(values) > 1 {
			lines = append(lines, "")
			lines = append(lines, graph(values, w, 3, 100, true)...)
			lines = append(lines, "")
		}
		add("Provider", g.Source)
		if g.UtilizationCounter != "" {
			add("Activity counter", g.UtilizationCounter)
		}
		if g.TemperatureSource != "" {
			add("Temperature source", g.TemperatureSource)
		}
		if g.TemperatureNote != "" {
			note(g.TemperatureNote)
		}
		for _, s := range g.TemperatureSensors {
			add(s.Key, fmt.Sprintf("%.1f°C", s.Celsius))
		}
		add("Scope", g.Scope)
		add("Identity", g.ID)
		lines = append(lines, "")
	}
	if len(r.Devices) == 0 {
		lines = append(lines, strong.Render("No accessible GPU"), "")
	}
	if !r.At.IsZero() {
		age := "demo"
		if !m.opts.Demo {
			age = fmt.Sprintf("%.0fs ago", max(0, time.Since(r.At).Seconds()))
		}
		add("Sampled every 5s", r.At.Local().Format("15:04:05")+" / "+age)
	}
	for _, problem := range r.Problems {
		note(problem)
	}
	return lines
}

func (m *Model) gpuCard(w, h int) []string {
	r := m.snapshot.GPU
	if len(r.Devices) == 0 {
		return []string{"", dim.Render("No accessible GPU")}
	}
	// Prefer measurable devices; Windows inventory rows are deliberately separate.
	g := r.Devices[0]
	for _, candidate := range r.Devices {
		if candidate.Utilization != nil {
			g = candidate
			break
		}
	}
	lines := []string{dim.Render(fit(Clean(g.Name), w))}
	label := "Utilization"
	if g.UtilizationCounter != "" {
		label = "Driver activity"
	}
	if g.Utilization != nil {
		lines = append(lines, pair(dim.Render(label), metric(fmt.Sprintf("%.1f%%", *g.Utilization), *g.Utilization), w), bar(*g.Utilization, w))
	} else {
		lines = append(lines, dim.Render("Utilization unavailable"))
	}
	if h >= 6 {
		lines = append(lines, "")
		if g.MemoryUsed != nil && g.MemoryTotal != nil {
			lines = append(lines, pair(dim.Render("VRAM"), bytes(*g.MemoryUsed)+" / "+bytes(*g.MemoryTotal), w))
		} else if g.MemoryUsed != nil {
			lines = append(lines, pair(dim.Render("Dedicated"), bytes(*g.MemoryUsed), w))
		} else if g.MemoryTotal != nil {
			lines = append(lines, pair(dim.Render("VRAM capacity"), bytes(*g.MemoryTotal), w))
		}
		if g.SharedUsed != nil {
			lines = append(lines, pair(dim.Render("Shared RAM"), bytes(*g.SharedUsed), w))
		}
		if g.MemoryUsed == nil && g.MemoryTotal == nil && g.SharedUsed == nil {
			lines = append(lines, pair(dim.Render("GPU memory"), "Unavailable", w))
		}
		if g.Temperature != nil {
			lines = append(lines, pair(dim.Render("Temperature"), fmt.Sprintf("%.0f°C", *g.Temperature), w))
		} else {
			lines = append(lines, pair(dim.Render("Temperature"), "Unavailable", w))
		}
		if g.Power != nil {
			lines = append(lines, pair(dim.Render("Power"), fmt.Sprintf("%.0f W", *g.Power), w))
		}
		if values := m.gpuHistory[g.Source+"/"+g.ID]; len(values) > 1 && h-len(lines) >= 2 {
			lines = append(lines, "")
			lines = append(lines, graph(values, w, h-len(lines), 100, true)...)
		}
	}
	return lines[:min(h, len(lines))]
}
func (m *Model) kind(panel int) int          { return m.kinds[panel] }
func (m *Model) panelTitle(panel int) string { return panels[m.kind(panel)] }
