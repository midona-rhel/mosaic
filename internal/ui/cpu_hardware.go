package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) cpuPowerLine(w int) string {
	r := m.snapshot.CPUHardware.Power
	value := "Unavailable"
	if watts := r.PackageWatts(); watts != nil {
		value = lipgloss.NewStyle().Foreground(cyan).Render(fmt.Sprintf("%.1f W", *watts))
	} else {
		// An unreadable sensor is not merely waiting for a second sample.
		packages, warming := 0, true
		for _, s := range r.Sensors {
			if s.Package {
				packages++
				if s.Watts == nil && s.State != "Sampling" {
					warming = false
				}
			}
		}
		if packages > 0 && warming {
			value = "Sampling…"
		}
	}
	label := "Package power"
	if w < 25 {
		label = "Power"
	}
	return pair(dim.Render(label), value, w)
}

func (m *Model) cpuHardwareSummary(w int) []string {
	r := m.snapshot.CPUHardware
	var lines []string
	if r.PhysicalCores > 0 && r.LogicalCPUs > 0 {
		lines = append(lines, detailPair("Cores / threads", fmt.Sprintf("%d / %d", r.PhysicalCores, r.LogicalCPUs), w)...)
	}
	if len(r.Clocks) > 0 {
		lo, hi := r.Clocks[0].MHz, r.Clocks[0].MHz
		for _, c := range r.Clocks {
			lo, hi = min(lo, c.MHz), max(hi, c.MHz)
		}
		lines = append(lines, detailPair("Reported clocks", fmt.Sprintf("%.0f–%.0f MHz", lo, hi), w)...)
	}
	return lines
}

func (m *Model) cpuHardwareDetails(w int) []string {
	r := m.snapshot.CPUHardware
	lines := []string{strong.Render("CPU POWER")}
	add := func(label, value string) { lines = append(lines, detailPair(label, value, w)...) }
	note := func(s string) {
		for _, line := range strings.Split(ansi.Wrap(Clean(s), max(1, w), ""), "\n") {
			lines = append(lines, dim.Render(line))
		}
	}
	if len(r.Power.Sensors) > 0 {
		for _, s := range r.Power.Sensors {
			value := "Unavailable"
			if s.State == "Sampling" {
				value = "Sampling energy delta…"
			}
			if s.Watts != nil {
				value = fmt.Sprintf("%.2f W", *s.Watts)
			}
			add(s.Name, value)
			if s.Interval > 0 {
				add("Measurement", fmt.Sprintf("Energy delta averaged over %.2fs", s.Interval.Seconds()))
			}
			for _, l := range s.Limits {
				value := fmt.Sprintf("%.1f W", l.Watts)
				if l.Window > 0 {
					value += fmt.Sprintf(" / %.2fs window", l.Window.Seconds())
				}
				add("Configured limit / "+l.Name, value)
			}
			add("Source", s.Source)
		}
	}
	for _, problem := range r.Power.Problems {
		note(problem)
	}
	if len(r.Power.Sensors) == 0 && len(r.Power.Problems) == 0 {
		note("CPU power unavailable: no accessible sensor.")
	}
	if !r.Power.At.IsZero() {
		age := "demo"
		if !m.opts.Demo {
			age = fmt.Sprintf("%.0fs ago", max(0, time.Since(r.Power.At).Seconds()))
		}
		add("Last power sample", r.Power.At.Local().Format("15:04:05")+" / "+age)
	}
	lines = append(lines, "", strong.Render("CLOCKS & PROCESSOR"))
	for _, c := range r.Clocks {
		add(c.Name, fmt.Sprintf("%.0f MHz", c.MHz))
		add("Reading", c.Kind)
		if c.MinMHz > 0 && c.MaxMHz > 0 {
			add("Configured frequency range", fmt.Sprintf("%.0f–%.0f MHz", c.MinMHz, c.MaxMHz))
		}
		if c.Governor != "" {
			add("Governor", c.Governor)
		}
		add("Source", c.Source)
	}
	seen := map[string]bool{}
	for _, info := range m.snapshot.CPUInfo {
		// One descriptor per package/model, rather than repeating it per thread.
		key := info.PhysicalID + "/" + info.ModelName
		if seen[key] {
			continue
		}
		seen[key] = true
		add("Processor", info.ModelName)
		if info.PhysicalID != "" {
			add("Package ID", info.PhysicalID)
		}
		if info.VendorID != "" {
			add("Vendor", info.VendorID)
		}
		if info.Family != "" || info.Model != "" {
			add("Family / model / stepping", fmt.Sprintf("%s / %s / %d", info.Family, info.Model, info.Stepping))
		}
		if info.Microcode != "" {
			add("Microcode", info.Microcode)
		}
		if strings.HasPrefix(info.ModelName, "Apple M") && info.Mhz > 0 && info.Mhz < 100 {
			add("OS reference frequency", "Unavailable")
		} else if info.Mhz > 0 {
			add("OS reference frequency", fmt.Sprintf("%.0f MHz", info.Mhz))
		}
		if len(info.Flags) > 0 {
			add("Reported CPU features", strings.Join(info.Flags, " "))
		}
	}
	for _, p := range r.Problems {
		note(p)
	}
	return lines
}
