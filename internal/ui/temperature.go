package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) cpuTemperatureLine(w int) string {
	label, value := "Temperature", "Unavailable"
	if sensor := m.snapshot.CPUThermal.Hottest(); sensor != nil {
		if sensor.Control {
			label = "Tctl control"
		}
		value = lipgloss.NewStyle().Foreground(cyan).Render(fmt.Sprintf("%.1f°C", sensor.Celsius))
		if sensor.Critical != nil && *sensor.Critical > 0 {
			value = metric(fmt.Sprintf("%.1f°C", sensor.Celsius), 100*sensor.Celsius / *sensor.Critical)
		}
	}
	if value == "Unavailable" && w < 24 {
		label = "Temp"
	}
	return pair(dim.Render(label), value, w)
}
func (m *Model) cpuTemperatureDetails(w int) []string {
	r := m.snapshot.CPUThermal
	lines := []string{strong.Render("CPU TEMPERATURE SENSORS")}
	note := func(s string) {
		for _, line := range strings.Split(ansi.Hardwrap(Clean(s), max(1, w), true), "\n") {
			lines = append(lines, dim.Render(line))
		}
	}
	if len(r.Sensors) > 0 {
		for _, s := range r.Sensors {
			label := s.Name
			if s.Control {
				label += " / control"
			}
			lines = append(lines, detailPair(label, fmt.Sprintf("%.1f°C", s.Celsius), w)...)
			if s.Critical != nil {
				lines = append(lines, detailPair("Reported critical limit", fmt.Sprintf("%.1f°C", *s.Critical), w)...)
			}
			lines = append(lines, detailPair("Source", s.Source, w)...)
		}
	} else {
		note("Temperature unavailable.")
	}
	if !r.At.IsZero() {
		age := "demo"
		if !m.opts.Demo {
			age = fmt.Sprintf("%.0fs ago", max(0, time.Since(r.At).Seconds()))
		}
		lines = append(lines, detailPair("Sampled every 5s", r.At.Local().Format("15:04:05")+" / "+age, w)...)
	}
	for _, p := range r.Problems {
		note(p)
	}
	return lines
}
