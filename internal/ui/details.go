package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) detailLines(w int) []string {
	s := m.snapshot
	lines := []string{}
	if s.At.IsZero() {
		return []string{dim.Render("Collecting…")}
	}
	add := func(label, value string) { lines = append(lines, detailPair(label, value, w)...) }
	switch m.kind(m.selected) {
	case 0:
		lines = m.systemContent(w, max(1, m.height-8), true)
	case 1:
		if len(s.CPUInfo) > 0 {
			for _, line := range strings.Split(ansi.Hardwrap(Clean(s.CPUInfo[0].ModelName), max(1, w), true), "\n") {
				lines = append(lines, strong.Render(line))
			}
		}
		lines = append(lines, m.cpuTemperatureLine(w), m.cpuPowerLine(w))
		lines = append(lines, m.cpuHardwareSummary(w)...)
		if !s.CPUReady {
			lines = append(lines, dim.Render("Warming up: CPU rates need two samples"))
		} else {
			add("Total utilization", fmt.Sprintf("%.1f%% across %d logical CPUs", average(s.CPU), len(s.CPU)))
			lines = append(lines, "")
			lines = append(lines, graph(m.cpuHistory, w, 3, 100, true)...)
			lines = append(lines, "")
			for i, v := range s.CPU {
				label, value := fmt.Sprintf("CPU %02d  ", i), fmt.Sprintf(" %6.1f%%", v)
				lines = append(lines, label+bar(v, max(1, w-len(label)-len(value)))+metric(value, v))
			}
		}
		lines = append(lines, "")
		lines = append(lines, m.cpuHardwareDetails(w)...)
		lines = append(lines, "")
		lines = append(lines, m.cpuTemperatureDetails(w)...)
	case 2:
		if v := s.Memory; v != nil {
			add("Physical memory", bytes(v.Total))
			add("Used", fmt.Sprintf("%s (%.1f%%)", bytes(v.Used), v.UsedPercent))
			lines = append(lines, "", bar(v.UsedPercent, w), "")
			add("Available", bytes(v.Available))
			add("Free", bytes(v.Free))
			if s.Host != nil && s.Host.OS == "linux" {
				add("Page cache / reclaimable", bytes(v.Cached))
			}
			if s.Host != nil && s.Host.OS == "darwin" {
				add("Wired", bytes(v.Wired))
				add("Active", bytes(v.Active))
				add("Inactive", bytes(v.Inactive))
			}
			lines = append(lines, "", spark(m.memHistory, w, 100), "")
		} else {
			lines = append(lines, dim.Render("Memory unavailable"))
		}
		if v := s.MemoryExtra; v != nil {
			add("Committed / limit", bytes(v.CommitUsed)+" / "+bytes(v.CommitLimit))
		}
		if v := s.Swap; v != nil {
			label := "Swap"
			if s.Host != nil && s.Host.OS == "windows" {
				label = "Pagefile"
			}
			add(label+" total", bytes(v.Total))
			add(label+" used", bytes(v.Used))
			add(label+" free", bytes(v.Free))
		} else {
			lines = append(lines, dim.Render("Swap unavailable"))
		}
	case 3:
		if len(visibleVolumes(s.Volumes)) == 0 {
			lines = append(lines, dim.Render("No accessible mounted filesystems"))
		}
		for _, v := range visibleVolumes(s.Volumes) {
			lines = append(lines, strong.Render(volumeName(v.Mount))+dim.Render("  "+Clean(v.Type)))
			add("Mount", v.Mount)
			add("Device", v.Device)
			add("Used / total", bytes(v.Used)+" / "+bytes(v.Total))
			lines = append(lines, "", bar(v.Percent, w), "")
			lines = append(lines, pair(dim.Render("Utilization"), metric(fmt.Sprintf("%.1f%%", v.Percent), v.Percent), w))
			add("Available", bytes(v.Free))

			lines = append(lines, "")
		}
		lines = append(lines, strong.Render("VOLUME / DEVICE I/O"))
		if len(s.Disks) == 0 {
			lines = append(lines, dim.Render("Warming up or device counters unavailable"))
		}
		for _, d := range s.Disks {
			add(d.Name, "R "+rate(d.Read)+" · W "+rate(d.Write))
		}
	case 7, 8, 9, 10:
		return m.extraDetails(m.kind(m.selected), w)
	case 6:
		return m.gpuLines(w)
	case 5:
		if !s.RatesReady {
			lines = append(lines, dim.Render("Warming up: traffic rates need two samples"))
		}
		down, up := traffic(s)
		add("Download (excluding loopback)", rate(down))
		add("Upload (excluding loopback)", rate(up))
		lines = append(lines, "", sparkWithHeat(m.netHistory, w, maxValue(m.netHistory), 60), "")
		for _, n := range s.Network {
			lines = append(lines, strong.Render(Clean(n.Name)))
			if n.Ready {
				add("Download / upload", rate(n.Down)+" / "+rate(n.Up))
			} else {
				add("Download / upload", "Warming up / counter reset")
			}
			add("Received / sent since boot", bytes(n.Received)+" / "+bytes(n.Sent))
			if info, ok := s.Interfaces[n.Name]; ok {
				add("Link flags / MTU", info.Flags+fmt.Sprintf(" / %d", info.MTU))
				if info.HardwareAddress != "" {
					add("Hardware address", info.HardwareAddress)
				}
				for _, address := range info.Addresses {
					add("Address", address)
				}
			}
			lines = append(lines, "")
		}
		if len(s.Network) == 0 {
			lines = append(lines, dim.Render("No accessible network counters"))
		}
	}
	problemKeys := []string{m.panelTitle(m.selected)}
	if m.kind(m.selected) == 2 {
		problemKeys = append(problemKeys, "Swap", "Memory extras")
	}
	if m.kind(m.selected) == 3 {
		problemKeys = append(problemKeys, "Disk I/O")
	}
	if m.kind(m.selected) == 5 {
		problemKeys = append(problemKeys, "Network interfaces")
	}
	for _, key := range problemKeys {
		if e, ok := s.Problems[key]; ok {
			lines = append(lines, "", lipgloss.NewStyle().Foreground(yellow).Render(key+": "+Clean(e)))
		}
	}
	return lines
}
func (m *Model) processDetailLines(w int) []string {
	if m.target == nil {
		return nil
	}
	if m.inspectError != "" {
		return []string{lipgloss.NewStyle().Foreground(yellow).Render(m.inspectError), "", dim.Render("Esc to close; refresh the process list")}
	}
	if m.inspection == nil {
		return []string{dim.Render("Reading process details…")}
	}
	d := m.inspection
	p := d.Process
	lines := []string{}
	add := func(label, value string) { lines = append(lines, detailPair(label, value, w)...) }
	parent := "Unavailable"
	if d.ParentReady {
		parent = fmt.Sprint(d.Parent)
	}
	add("PID / parent", fmt.Sprintf("%d / %s", p.PID, parent))
	add("User", d.User)
	add("Status", d.Status)
	threads := "Unavailable"
	if d.ThreadsReady {
		threads = fmt.Sprint(d.Threads)
	}
	add("Threads", threads)
	add("CPU", cpuLabel(p)+"%")
	rss := "Unavailable"
	if p.MemoryReady {
		rss = bytes(p.RSS)
	}
	add("Resident memory", rss)
	started := "Unavailable"
	if p.Started > 0 {
		started = time.UnixMilli(p.Started).Local().Format(time.RFC1123)
	}
	add("Started", started)
	lines = append(lines, "", dim.Render("Executable"))
	lines = append(lines, strings.Split(ansi.Hardwrap(Clean(d.Executable), max(1, w), true), "\n")...)
	lines = append(lines, "", dim.Render("Command"))
	lines = append(lines, strings.Split(ansi.Hardwrap(Clean(d.Command), max(1, w), true), "\n")...)
	for _, e := range d.Problems {
		lines = append(lines, lipgloss.NewStyle().Foreground(yellow).Render(Clean(e)))
	}
	return lines
}
