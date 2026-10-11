package ui

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	ink        = lipgloss.Color("#E9E8F5")
	muted      = lipgloss.Color("#87859D")
	accent     = lipgloss.Color("#B45CFF")
	cyan       = lipgloss.Color("#538FFF")
	green      = cyan
	yellow     = accent
	red        = lipgloss.Color("#FF456D")
	edge       = lipgloss.Color("#36334E")
	background = lipgloss.Color("#100E19")
	textStyle  = lipgloss.NewStyle().Foreground(ink)
	dim        = lipgloss.NewStyle().Foreground(muted)
	strong     = lipgloss.NewStyle().Foreground(ink).Bold(true)
)

// Clean ensures OS-provided names and commands cannot inject terminal controls.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 0x7f {
			return ' '
		}
		return r
	}, ansi.Strip(s))
}
func fit(s string, w int) string { return ansi.Truncate(s, max(0, w), "…") }
func pad(s string, w int) string {
	s = fit(s, w)
	return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
}
func pair(left, right string, w int) string {
	right = fit(right, max(0, w-min(lipgloss.Width(left), max(6, w/3))-1))
	lw := w - lipgloss.Width(right) - 1
	return pad(left, max(0, lw)) + " " + right
}
func bytes(v uint64) string { return byteFloat(float64(v)) }
func byteFloat(v float64) string {
	u := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	i := 0
	for v >= 1024 && i < len(u)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, u[i])
	}
	return fmt.Sprintf("%.1f %s", v, u[i])
}
func rate(v float64) string { return byteFloat(v) + "/s" }
func duration(seconds uint64) string {
	d := seconds / 86400
	h := seconds % 86400 / 3600
	m := seconds % 3600 / 60
	if d > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", d, h, m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}
func bar(value float64, w int) string {
	return progressBar(value, w, os.Getenv("UNICODE_PROGRESS_BAR") != "false")
}

// Fira Code's documented U+EE00..EE05 progress glyphs join across cells.
// Default to Fira Code; UNICODE_PROGRESS_BAR=false selects portable meters.
func progressBar(value float64, w int, fira bool) string {
	return meter(value, w, fira, -1)
}
func chargeBar(value float64, w int) string {
	return meter(value, w, os.Getenv("UNICODE_PROGRESS_BAR") != "false", 100-value)
}
func meter(value float64, w int, fira bool, heat float64) string {
	if w <= 0 {
		return ""
	}
	if w == 1 {
		return dim.Render("[")
	}
	if w == 2 {
		return dim.Render("[]")
	}
	capacity := w - 2
	if fira {
		capacity = w
	}
	n := int(math.Round(math.Max(0, math.Min(100, value)) * float64(capacity) / 100))
	var b strings.Builder
	if !fira {
		b.WriteString(dim.Render("["))
	}
	for i := range capacity {
		// Color both glyph and cell background in the portable meter so the
		// track and fill have the same height, independent of font metrics.
		// Shading keeps empty cells distinguishable when colors are stripped.
		glyph := "░"
		c := edge
		filled := i < n
		if fira {
			r := rune(0xEE01)
			if i == 0 {
				r = 0xEE00
			} else if i == capacity-1 {
				r = 0xEE02
			}
			if filled {
				r += 3
			}
			glyph = string(r)
		} else if filled {
			glyph = "█"
		}
		if filled {
			v := 100 * float64(i+1) / float64(capacity)
			if heat >= 0 {
				v = heat * float64(i+1) / float64(max(1, n))
			}
			c = lipgloss.Color(usageHex(v))
		}
		style := lipgloss.NewStyle().Foreground(c)
		if !fira {
			style = style.Background(c)
		}
		b.WriteString(style.Render(glyph))
	}
	if !fira {
		b.WriteString(dim.Render("]"))
	}
	return b.String()
}

// Usage shares one continuous blue -> neon purple -> red scale across resources.
func usageHex(value float64) string {
	v := math.Max(0, math.Min(100, value))
	if v <= 35 {
		return "#538FFF"
	}
	a, b, t := [3]float64{83, 143, 255}, [3]float64{180, 92, 255}, (v-35)/25
	if v > 60 {
		a, b, t = b, [3]float64{255, 69, 109}, (v-60)/40
	}
	return fmt.Sprintf("#%02X%02X%02X", int(math.Round(a[0]+(b[0]-a[0])*t)), int(math.Round(a[1]+(b[1]-a[1])*t)), int(math.Round(a[2]+(b[2]-a[2])*t)))
}
func usageColor(v float64) color.Color { return lipgloss.Color(usageHex(v)) }
func metric(s string, v float64) string {
	return lipgloss.NewStyle().Foreground(usageColor(v)).Bold(true).Render(s)
}
func spark(values []float64, w int, ceiling float64) string {
	return sparkWithHeat(values, w, ceiling, 100)
}
func sparkWithHeat(values []float64, w int, ceiling, heatLimit float64) string {
	if len(values) == 0 {
		return dim.Render("Waiting for samples")
	}
	values = values[max(0, len(values)-w):]
	chars := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range values {
		idx := int(math.Round(math.Max(0, math.Min(1, v/math.Max(1, ceiling))) * 7))
		b.WriteString(lipgloss.NewStyle().Foreground(usageColor(heatLimit * v / math.Max(1, ceiling))).Render(string(chars[idx])))
	}
	return b.String()
}
func graph(values []float64, w, h int, ceiling float64, utilization bool) []string {
	if h <= 0 {
		return nil
	}
	out := make([]string, h)
	glyphs := []rune(" ▁▂▃▄▅▆▇█")
	start := max(0, len(values)-w)
	vs := values[start:]
	padX := max(0, w-len(vs))
	for y := range h {
		var row strings.Builder
		row.WriteString(strings.Repeat(" ", padX))
		for _, v := range vs {
			ratio := math.Max(0, math.Min(1, v/math.Max(1, ceiling)))
			units := int(math.Round(ratio*float64(h*8))) - (h-1-y)*8
			units = min(8, max(0, units))
			c := usageColor(ratio * 100)
			if !utilization {
				c = usageColor(ratio * 60)
			}
			if units == 8 {
				// Filled terminal cells have no font-dependent gaps between columns.
				row.WriteString(lipgloss.NewStyle().Foreground(c).Background(c).Render("█"))
			} else {
				row.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(glyphs[units])))
			}
		}
		out[y] = row.String()
	}
	return out
}
func fixedBlock(lines []string, w, h int) string {
	out := make([]string, max(0, h))
	for i := range out {
		if i < len(lines) {
			out[i] = pad(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", max(0, w))
		}
	}
	return strings.Join(out, "\n")
}
func box(lines []string, w, h int, color string) string {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(color)).
		BorderBackground(background).Foreground(ink).Background(background).Padding(0, 1)
	if color == "#B45CFF" || color == "#538FFF" {
		style = style.BorderForegroundBlend(cyan, accent, lipgloss.Color("#E5A1FF"), accent, cyan)
	}
	if color == "#FF456D" {
		style = style.BorderForegroundBlend(accent, red, lipgloss.Color("#FFA4B9"), red, accent)
	}
	return style.Render(fixedBlock(lines, max(0, w-4), max(0, h-2)))
}

func badge(label string, color color.Color) string {
	return lipgloss.NewStyle().Foreground(ink).Background(lipgloss.Darken(color, .58)).Bold(true).Render(" " + label + " ")
}
