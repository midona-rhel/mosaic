package ui

import (
	"fmt"
	"path"
	"strings"

	"github.com/midona-rhel/mosaic/internal/metrics"
)

// OS names can contain either separator, independently of the host running
// this UI. Keep full paths in inspection; use their final component in lists.
func shortName(value string) string {
	value = strings.TrimSpace(Clean(value))
	if value == "" {
		return "—"
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, "./") || strings.HasPrefix(normalized, "../") || strings.Contains(value, "\\") || (len(normalized) > 2 && normalized[1] == ':' && normalized[2] == '/') {
		return path.Base(normalized)
	}
	// Kernel thread names such as kworker/0:1 contain slashes but are names.
	return value
}

func volumeName(mount string) string {
	if mount == "/" {
		return "System"
	}
	return shortName(mount)
}

func overviewProcessHeader(w int) string {
	if w < 28 {
		if w < 22 {
			return pair(dim.Render("NAME"), dim.Render("CPU %"), w)
		}
		return pad(dim.Render("NAME"), w-14) + " " + pad(dim.Render("CPU %"), 6) + " " + pad(dim.Render("MEM"), 6)
	}
	prefix := ""
	if w >= 54 {
		prefix = pad(dim.Render("PID"), 7) + "  "
		w -= 9
	}
	return prefix + pad(dim.Render("NAME"), w-18) + " " + pad(dim.Render("CPU %"), 7) + " " + pad(dim.Render("MEMORY"), 9)
}

func overviewProcessLine(p metrics.Process, w int) string {
	cpu := "—"
	if p.CPUReady {
		cpu = fmt.Sprintf("%.1f%%", p.CPU)
	}
	if w < 28 {
		if w < 22 {
			return pair(shortName(p.Name), metric(cpu, min(100, p.CPU)), w)
		}
		memory := "—"
		if p.MemoryReady {
			memory = fmt.Sprintf("%.0fM", float64(p.RSS)/(1<<20))
			if p.RSS >= 1<<30 {
				memory = fmt.Sprintf("%.1fG", float64(p.RSS)/(1<<30))
			}
		}
		return pad(shortName(p.Name), w-14) + " " + pad(metric(cpu, min(100, p.CPU)), 6) + " " + pad(dim.Render(memory), 6)
	}
	prefix := ""
	if w >= 54 {
		prefix = pad(dim.Render(fmt.Sprint(p.PID)), 7) + "  "
		w -= 9
	}
	memory := "—"
	if p.MemoryReady {
		memory = bytes(p.RSS)
	}
	return prefix + pad(shortName(p.Name), w-18) + " " + pad(metric(cpu, min(100, p.CPU)), 7) + " " + pad(dim.Render(memory), 9)
}
