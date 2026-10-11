package ui

import (
	"fmt"
	"strings"

	"github.com/midona-rhel/mosaic/internal/metrics"
)

func busiestDisk(s metrics.Snapshot) *metrics.IO {
	var best *metrics.IO
	for i := range s.Disks {
		d := &s.Disks[i]
		if best == nil || d.Read+d.Write > best.Read+best.Write {
			best = d
		}
	}
	return best
}
func optional(v *float64, format string) string {
	if v == nil {
		return "Unavailable"
	}
	return fmt.Sprintf(format, *v)
}
func batteryCapacity(b metrics.Battery) string {
	if b.Remaining == nil || b.Full == nil || *b.Full <= 0 || b.CapacityUnit == "" {
		return "Unavailable"
	}
	if b.CapacityUnit == "mAh" {
		return fmt.Sprintf("%.0f / %.0f mAh", *b.Remaining, *b.Full)
	}
	return fmt.Sprintf("%.1f / %.1f %s", *b.Remaining, *b.Full, b.CapacityUnit)
}
func (m *Model) diskSeries(name string) ([]float64, []float64) {
	v := m.diskHistory[name]
	return v[0], v[1]
}
func (m *Model) extraCard(kind, w, h int) []string {
	s := m.snapshot
	lines := []string{}
	add := func(k, v string) { lines = append(lines, pair(dim.Render(k), v, w)) }
	switch kind {
	case 7:
		if s.Problems["Disk I/O"] != "" {
			return []string{dim.Render("Disk I/O unavailable")}
		}
		d := busiestDisk(s)
		if !s.DiskReady {
			return []string{dim.Render("Warming up…")}
		}
		if d == nil {
			return []string{dim.Render("No device counters")}
		}
		rh, wh := m.diskSeries(d.Name)
		nameRows := 0
		if h >= 7 {
			nameRows = 1
			lines = append(lines, dim.Render(fit(Clean(d.Name), w)))
		}
		gh := max(1, (h-3-nameRows)/2)
		add("Read", rate(d.Read))
		if h >= 5 {
			lines = append(lines, graph(rh, w, gh, maxValue(rh), false)...)
		}
		if h >= 5 {
			lines = append(lines, "")
		}
		add("Write", rate(d.Write))
		if h >= 5 {
			lines = append(lines, graph(wh, w, gh, maxValue(wh), false)...)
		}
	case 8:
		if !s.Extras.Collected.Battery {
			return []string{dim.Render("Collecting battery…")}
		}
		r := s.Extras.Battery
		if len(r.Devices) == 0 {
			if r.Problem != "" {
				return []string{dim.Render("Battery unavailable")}
			}
			return []string{dim.Render("No system battery")}
		}
		b := r.Devices[0]
		add(Clean(b.State), optional(b.Percent, "%.0f%%"))
		add("Left", batteryCapacity(b))
		if b.Percent != nil && h >= 5 {
			lines = append(lines, chargeBar(*b.Percent, w), "")
		}
		add("Power", optional(b.Watts, "%.1f W"))
		if h >= 6 {
			add("Cycles", optional(b.Cycles, "%.0f"))
			add("Voltage", optional(b.Voltage, "%.2f V"))
		}
	case 9:
		lines = append(lines, m.cpuTemperatureLine(w))
		gpu := "Unavailable"
		for _, d := range s.GPU.Devices {
			if d.Temperature != nil {
				gpu = optional(d.Temperature, "%.1f°C")
				break
			}
		}
		// The CPU helper says Temperature; identify it explicitly in this panel.
		if cpu := s.CPUThermal.Hottest(); cpu != nil {
			label := "CPU"
			if cpu.Control {
				label = "CPU control"
			}
			lines[0] = pair(dim.Render(label), fmt.Sprintf("%.1f°C", cpu.Celsius), w)
		} else {
			lines[0] = pair(dim.Render("CPU"), "Unavailable", w)
		}
		add("GPU", gpu)
		if len(s.Extras.Fans.Devices) == 0 {
			add("Fans", "Unavailable")
		}
		for _, f := range s.Extras.Fans.Devices {
			if len(lines) >= h {
				break
			}
			add(Clean(f.Name), fmt.Sprintf("%.0f RPM", f.RPM))
		}
	case 10:
		if !s.Extras.Collected.Connections {
			return []string{dim.Render("Collecting connections…")}
		}
		r := s.Extras.Connections
		if r.Problem != "" && len(r.Entries) == 0 {
			return []string{dim.Render("Connections unavailable")}
		}
		listening := 0
		for _, v := range r.Entries {
			if v.State == "LISTEN" {
				listening++
			}
		}
		add(fmt.Sprintf("%d sockets", len(r.Entries)), fmt.Sprintf("%d listen", listening))
		if h > 4 {
			lines = append(lines, dim.Render(pair("PROCESS", "LOCAL PORT", w)))
		}
		for _, c := range r.Entries {
			if len(lines) >= h {
				break
			}
			port := c.Local
			if i := strings.LastIndex(port, ":"); i >= 0 {
				port = port[i+1:]
			}
			name := Clean(c.Process)
			if name == "Unavailable" && c.PID > 0 {
				name = fmt.Sprintf("PID %d", c.PID)
			}
			add(shortName(name), c.Protocol+" "+port)
		}
	}
	return lines[:min(len(lines), h)]
}
func (m *Model) extraDetails(kind, w int) []string {
	s := m.snapshot
	lines := []string{}
	add := func(k, v string) { lines = append(lines, detailPair(Clean(k), v, w)...) }
	section := func(title string) {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, strong.Render(title))
	}
	switch kind {
	case 7:
		if !s.DiskReady {
			lines = append(lines, dim.Render("Warming up: disk rates require two samples."))
		}
		if len(s.Disks) == 0 {
			add("Device counters", "Unavailable")
		}
		if e := s.Problems["Disk I/O"]; e != "" {
			add("Provider", e)
		}
		for _, d := range s.Disks {
			section(Clean(d.Name))
			rh, wh := m.diskSeries(d.Name)
			add("Read", rate(d.Read))
			lines = append(lines, graph(rh, w, 2, maxValue(rh), false)...)
			lines = append(lines, "")
			add("Write", rate(d.Write))
			lines = append(lines, graph(wh, w, 2, maxValue(wh), false)...)
		}
		add("Source", "gopsutil device I/O counters: Linux procfs, macOS IOKit, Windows disk performance counters.")
	case 8:
		if !s.Extras.Collected.Battery {
			return []string{dim.Render("Collecting battery…")}
		}
		r := s.Extras.Battery
		if len(r.Devices) == 0 {
			add("Battery", "No system battery reported")
		}
		for _, b := range r.Devices {
			section(Clean(b.Name))
			add("State", b.State)
			add("Charge", optional(b.Percent, "%.1f%%"))
			add("Remaining / full", batteryCapacity(b))
			if b.Percent != nil {
				lines = append(lines, chargeBar(*b.Percent, w), "")
			}
			add("Battery power", optional(b.Watts, "%.2f W"))
			add("Capacity / design", optional(b.Health, "%.1f%%"))
			add("Cycle count", optional(b.Cycles, "%.0f"))
			add("Voltage", optional(b.Voltage, "%.3f V"))
			add("Source", b.Source)
			if b.PowerSource != "" {
				add("Power source", b.PowerSource)
			}
			if b.PowerProblem != "" {
				add("Power reading", b.PowerProblem)
			}
		}
		if r.Problem != "" {
			add("Provider", r.Problem)
		}
	case 9:
		section("CPU TEMPERATURES")
		if len(s.CPUThermal.Sensors) == 0 {
			add("CPU", "Unavailable")
			for _, p := range s.CPUThermal.Problems {
				add("CPU provider", p)
			}
		}
		for _, v := range s.CPUThermal.Sensors {
			label := v.Name
			if v.Control {
				label += " (control sensor)"
			}
			add(label, fmt.Sprintf("%.1f°C", v.Celsius))
			add("Source", v.Source)
		}
		section("GPU TEMPERATURES")
		if len(s.GPU.Devices) == 0 {
			add("GPU", "Unavailable")
			for _, p := range s.GPU.Problems {
				add("GPU provider", p)
			}
		}
		for _, v := range s.GPU.Devices {
			add(v.Name, optional(v.Temperature, "%.1f°C"))
			if v.TemperatureSource != "" {
				add("Source", v.TemperatureSource)
			}
		}
		section("FAN SPEEDS")
		r := s.Extras.Fans
		for _, f := range r.Devices {
			add(f.Name, fmt.Sprintf("%.0f RPM", f.RPM))
			add("Source", f.Source)
		}
		if len(r.Devices) == 0 {
			add("Fans", "Unavailable / no exposed fan counters")
		}
		if r.Problem != "" {
			add("Provider", r.Problem)
		}
	case 10:
		if !s.Extras.Collected.Connections {
			return []string{dim.Render("Collecting connections…")}
		}
		r := s.Extras.Connections
		add("Reported sockets", fmt.Sprint(len(r.Entries)))
		add("Source", r.Source)
		add("Visibility", "Accessible TCP/UDP sockets only. Other users' processes may be omitted or have no owner. No DNS lookups or network probing.")
		if r.Problem != "" {
			add("Provider", r.Problem)
		}
		for _, c := range r.Entries {
			name := Clean(c.Process)
			if c.PID > 0 {
				name += fmt.Sprintf(" · PID %d", c.PID)
			}
			section(c.Protocol + " · " + c.State + " · " + name)
			add("Local", c.Local)
			add("Remote", c.Remote)
		}
	}

	return lines
}

// Compact titles reserve room for the stable number and pop-out button.
func (m *Model) cardTitle(panel int) string {
	if m.kind(panel) == 7 {
		return "Disk I/O"
	}
	return m.panelTitle(panel)
}
