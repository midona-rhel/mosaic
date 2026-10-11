package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/midona-rhel/mosaic/internal/metrics"
)

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	// Newly exposed cells and terminal padding use this color before our next
	// resize frame arrives. Bubble Tea resets the terminal default on shutdown.
	if !m.opts.NoTerminalEffects {
		v.BackgroundColor = background
		v.WindowTitle = "Mosaic · system monitor"
	}
	if !m.opts.NoMouse {
		v.MouseMode = tea.MouseModeAllMotion
	}
	return v
}
func (m *Model) render() string {
	w, h := m.width, m.height
	canvas := lipgloss.NewCanvas(w, h)
	base := m.dragContent("base", w, h, func() string { return lipgloss.NewStyle().Background(background).Render(fixedBlock(nil, w, h)) })
	layers := []*lipgloss.Layer{lipgloss.NewLayer(base)}
	if w < 24 || h < 10 {
		lines := []string{strong.Render("MOSAIC"), "", dim.Render("Resize to at least 24×10"), dim.Render("q quit · Ctrl+C quit")}
		layers = append(layers, lipgloss.NewLayer(fixedBlock(lines, w, h)).Z(1))
		m.compose(canvas, layers)
		return m.finishCanvas(canvas)
	}
	if !m.expanded || m.kind(m.selected) == 0 {
		l := m.layout()
		if len(m.visiblePanels()) == 0 {
			layers = append(layers, lipgloss.NewLayer(dim.Render(fit("No panels · press o to add panels", w-4))).X(2).Y(3).Z(1))
		}
		for slot, r := range l.Cards {
			if r.W == 0 {
				continue
			}
			i := m.order[slot]
			color := "#36334E"
			if i == m.selected {
				color = "#B45CFF"
			}
			layers = append(layers, lipgloss.NewLayer(m.dragContent(fmt.Sprintf("card-%d", i), r.W, r.H, func() string { return box(m.cardLines(i, r.W-4, r.H-2), r.W, r.H, color) })).X(r.X).Y(r.Y).Z(1).ID(fmt.Sprintf("card-%d", i)))
		}
		layers = append(layers, m.floatingLayers()...)
	}
	if m.expanded {
		r := m.expandedRect()
		as := m.expandedActions()
		lines := []string{pair(lipgloss.NewStyle().Foreground(accent).Bold(true).Render(strings.ToUpper(m.panelTitle(m.selected))), m.control(as[0])+" "+m.control(as[1]), r.W-4), ""}
		if m.kind(m.selected) == 4 {
			lines = append(lines, m.processTable(r.W-4, r.H-4)...)
		} else {
			all := m.detailLines(r.W - 4)
			start := min(m.offset, len(all))
			end := min(len(all), start+r.H-4)
			lines = append(lines, all[start:end]...)
		}
		layers = append(layers, lipgloss.NewLayer(box(lines, r.W, r.H, "#B45CFF")).X(r.X).Y(r.Y).Z(29))
	}
	// Header and footer stay fixed; the dashboard never extends below them.
	layers = append(layers, lipgloss.NewLayer(m.header()).Z(30), lipgloss.NewLayer(m.footer()).Y(h-2).Z(30))
	if m.organizing {
		r := m.organizerRect()
		layers = append(layers, m.overlay(m.organizerLines(r.W-4), r, "#B45CFF")...)
	} else if m.picking {
		r := m.pickerRect()
		layers = append(layers, m.overlay(m.pickerLines(r.W-4), r, "#B45CFF")...)
	} else if m.help {
		r := m.modalRect()
		all := m.helpLines()
		start := min(m.helpOffset, len(all))
		end := min(len(all), start+max(0, r.H-2))
		layers = append(layers, m.overlay(all[start:end], r, "#B45CFF")...)
	} else if m.target != nil {
		r := m.inspectorRect()
		all := m.processDetailLines(r.W - 4)
		as := m.inspectorActions()
		title := []string{pair(strong.Render("PROCESS / "+shortName(m.target.Name)), m.control(as[0]), r.W-4), ""}
		start := min(m.modalOffset, len(all))
		end := min(len(all), start+max(0, r.H-6))
		title = append(title, all[start:end]...)
		for len(title) < r.H-3 {
			title = append(title, "")
		}
		if len(as) > 1 {
			title = append(title, m.control(as[1]))
		}
		layers = append(layers, m.overlay(title, r, "#538FFF")...)
	}
	if m.confirm && m.target != nil {
		r := m.confirmationRect()
		lines := []string{lipgloss.NewStyle().Foreground(red).Bold(true).Render("TERMINATE PROCESS?"), "", shortName(m.target.Name) + fmt.Sprintf(" · PID %d", m.target.PID), "Unsaved work may be lost.", "", dim.Render("Unix: SIGTERM · Windows: terminate")}
		if r.H < 10 {
			lines = []string{strong.Render("TERMINATE?"), shortName(m.target.Name) + fmt.Sprintf(" · %d", m.target.PID), "Unsaved work may be lost."}
		}
		as := m.confirmationActions()
		buttonRows := 1
		if as[0].Rect.Y != as[1].Rect.Y {
			buttonRows = 2
		}
		lines = lines[:min(len(lines), r.H-2-buttonRows)]
		for len(lines) < r.H-2-buttonRows {
			lines = append(lines, "")
		}
		if buttonRows == 2 {
			lines = append(lines, m.control(as[0]), m.control(as[1]))
		} else {
			lines = append(lines, m.control(as[0])+" "+m.control(as[1]))
		}
		if m.terminating {
			lines = []string{strong.Render("Requesting termination…"), dim.Render("Waiting for the operating system")}
		}
		ls := m.overlay(lines, r, "#FF456D")
		for i, l := range ls {
			l.Z(60 + i)
		}
		layers = append(layers, ls...)
	}
	m.compose(canvas, layers)
	return m.finishCanvas(canvas)
}

// Each cell has a real background, including nested ANSI runs and borders.
// Emphasis dims the workspace itself; it does not add offset shadow rectangles.
func (m *Model) finishCanvas(canvas *lipgloss.Canvas) string {
	hoverRect, hoverEdge := m.hoveredEdge()
	hoverOwner := m.hoverOwnerAt(m.mouseX, m.mouseY)
	hoverSurface := m.hoveredSurface()
	var spotlight *Rect
	if m.expanded && m.kind(m.selected) == 0 {
		r := m.expandedRect()
		spotlight = &r
	}
	if m.organizing {
		r := m.organizerRect()
		spotlight = &r
	}
	if m.picking {
		r := m.pickerRect()
		spotlight = &r
	}
	if m.help {
		r := m.modalRect()
		spotlight = &r
	}
	if m.target != nil && !m.help {
		r := m.inspectorRect()
		spotlight = &r
	}
	if m.confirm && m.target != nil {
		r := m.confirmationRect()
		spotlight = &r
	}
	for y := range canvas.Height() {
		for x := range canvas.Width() {
			cell := canvas.CellAt(x, y)
			if cell == nil {
				continue
			}
			if cell.Style.Bg == nil {
				cell.Style.Bg = background
			}
			if cell.Style.Fg == nil {
				cell.Style.Fg = ink
			}
			if edgesAt(hoverSurface, x, y) != 0 && m.hoverOwnerAt(x, y) == hoverOwner {
				cell.Style.Fg = lipgloss.Color("#7799BF")
			}
			if m.drag == nil && onHighlightedEdge(hoverRect, hoverEdge, x, y) && m.hoverOwnerAt(x, y) == hoverOwner {
				cell.Style.Fg = lipgloss.Color("#73D2FF")

			}
			if spotlight != nil && !spotlight.Contains(x, y) && y < m.height-2 {
				cell.Style.Fg = lipgloss.Darken(cell.Style.Fg, .5)
			}
			canvas.SetCell(x, y, cell)
		}
	}
	m.paintDragPreview(canvas)
	m.paintScrollbars(canvas)
	return canvas.Render()
}
func (m *Model) header() string {
	mode := ""
	color := green
	if m.opts.Demo {
		mode = "◇ DEMO"
		color = yellow
	}
	if m.paused {
		mode = "Ⅱ PAUSED"
		color = yellow
	}
	left := badge("MOSAIC", accent) + dim.Render(" // SYSTEM MONITOR")
	right := lipgloss.NewStyle().Foreground(color).Render(mode)
	host := "Collecting system information…"
	if s := m.snapshot.Host; s != nil {
		host = fmt.Sprintf("%s · %s · %s · up %s", Clean(s.Hostname), Clean(s.Platform), Clean(s.KernelArch), duration(s.Uptime))
	}
	if m.opts.ReadOnly {
		host += "  /  read-only"
	}
	return lipgloss.NewStyle().Background(background).Render(fixedBlock([]string{" " + pair(left, right, m.width-2), " " + dim.Render(fit(host, m.width-2))}, m.width, 2))
}
func (m *Model) footer() string {
	status := ""

	if status == "" && (strings.Contains(m.status, "failed") || strings.Contains(m.status, "disabled")) {
		status = m.status
	}
	if m.drag == nil && !m.expanded && !m.organizing && !m.help && !m.picking && m.target == nil {
		hidden := m.previewHidden()
		if len(hidden) > 0 {
			status = "o panels · Hidden: " + strings.Join(hidden, ", ")
		}
	}
	if status == "" && len(m.snapshot.Problems) > 0 {
		status = fmt.Sprintf("%d metrics unavailable", len(m.snapshot.Problems))
	}
	_, buttons := m.actionBar()
	status = fit(status, m.width-2)
	return lipgloss.NewStyle().Background(background).Render(fixedBlock([]string{strings.Repeat(" ", max(0, m.width-1-lipgloss.Width(status))) + dim.Render(status), buttons}, m.width, 2))
}
func (m *Model) cardLines(i, w, h int) []string {
	mark := " "
	if i == m.selected {
		mark = "›"
	}
	label := strong.Render(strings.ToUpper(m.cardTitle(i)))
	if m.reordering {
		span, limits := m.span(i), m.spanLimits()
		label += dim.Render(fmt.Sprintf("  %d×%d", min(span.W, limits.W), min(span.H, limits.H)))
	}
	prefix := fmt.Sprintf("%d ", i+1)
	left := lipgloss.NewStyle().Foreground(cyan).Render(mark+" ") + dim.Render(prefix) + label
	if lipgloss.Width(left) > w-8 {
		left = dim.Render(prefix) + label
	}
	if lipgloss.Width(left) > w-8 {
		left = label
	}
	title := pair(left, m.control(m.cardHide(i))+" "+m.control(m.cardPopout(i)), w)
	lines := []string{title, ""}
	s := m.snapshot
	if s.At.IsZero() {
		return append(lines, dim.Render("Collecting…"))
	}
	roomy := h >= 8
	switch m.kind(i) {
	case 0:
		lines = append(lines[:1], m.systemContent(w, h-1, false)...)
	case 1:
		lines = lines[:1]
		if s.CPUReady {
			v := average(s.CPU)
			lines = append(lines, pair(metric(fmt.Sprintf("%.1f%%", v), v), dim.Render(fmt.Sprintf("%d CPUs", len(s.CPU))), w))
			if roomy {
				lines = append(lines, bar(v, w), "")
			}
			lines = append(lines, m.cpuTemperatureLine(w), m.cpuPowerLine(w))
			if roomy {
				cols := max(1, w/30)
				gh := h - 8
				if h >= 16 {
					coreRows := min((len(s.CPU)+cols-1)/cols, h-12)
					gh = max(3, h-10-coreRows)
				}
				lines = append(lines, graph(m.cpuHistory, w, gh, 100, true)...)
				lines = append(lines, "")
				if len(s.CPUInfo) > 0 {
					lines = append(lines, dim.Render(Clean(s.CPUInfo[0].ModelName)))
				}
				if len(lines)+2 < h {
					lines = append(lines, "", dim.Render("PER CORE"))
					cols := max(1, w/30)
					cw := (w - (cols-1)*2) / cols
					for j := 0; j < len(s.CPU) && len(lines) < h; j += cols {
						parts := []string{}
						for k := j; k < min(len(s.CPU), j+cols); k++ {
							parts = append(parts, pad(fmt.Sprintf("%02d ", k)+bar(s.CPU[k], max(1, cw-10))+metric(fmt.Sprintf(" %5.1f%%", s.CPU[k]), s.CPU[k]), cw))
						}
						lines = append(lines, strings.Join(parts, "  "))
					}
				}
			} else {
				if h > len(lines) {
					lines = append(lines, graph(m.cpuHistory, w, h-len(lines), 100, true)...)
				}
			}
		} else {
			lines = append(lines, strong.Render("Warming up…"), m.cpuTemperatureLine(w), m.cpuPowerLine(w))
		}
	case 2:
		if v := s.Memory; v != nil {
			lines = append(lines, pair(metric(fmt.Sprintf("%.1f%%", v.UsedPercent), v.UsedPercent), dim.Render(bytes(v.Total)+" total"), w), bar(v.UsedPercent, w), "")
			if h >= 7 {
				lines = append(lines, pair(dim.Render("Used"), bytes(v.Used), w), pair(dim.Render("Available"), bytes(v.Available), w))
				if s.Swap != nil {
					label := "Swap"
					if s.Host != nil && s.Host.OS == "windows" {
						label = "Pagefile"
					}
					lines = append(lines, pair(dim.Render(label), bytes(s.Swap.Used)+" / "+bytes(s.Swap.Total), w))
				}
				extra := []string{}
				if h >= 16 {
					extra = append(extra, pair(dim.Render("Free"), bytes(v.Free), w))
					if s.Host != nil {
						switch s.Host.OS {
						case "linux":
							extra = append(extra, pair(dim.Render("Cached"), bytes(v.Cached), w))
						case "darwin":
							extra = append(extra, pair(dim.Render("Wired"), bytes(v.Wired), w), pair(dim.Render("Active"), bytes(v.Active), w), pair(dim.Render("Inactive"), bytes(v.Inactive), w))
						case "windows":
							if s.MemoryExtra != nil {
								extra = append(extra, pair(dim.Render("Committed / limit"), bytes(s.MemoryExtra.CommitUsed)+" / "+bytes(s.MemoryExtra.CommitLimit), w))
							}
						}
					}
				}
				if h-len(lines) >= 2 {
					lines = append(lines, graph(m.memHistory, w, max(1, h-len(lines)-1-len(extra)), 100, true)...)
					lines = append(lines, "")
				}
				lines = append(lines, extra...)
			}
		} else {
			lines = append(lines, dim.Render("Unavailable"))
		}
	case 3:
		lines = lines[:1]
		vs := visibleVolumes(s.Volumes)
		start := m.diskStart(i, diskCardCount(h))
		if len(vs) > 0 {
			if h < 6 {
				v := vs[start]
				lines = append(lines, pair(volumeName(v.Mount), metric(fmt.Sprintf("%.1f%%", v.Percent), v.Percent), w), bar(v.Percent, w))
			} else {
				count := min(len(vs)-start, diskCardCount(h))
				for _, v := range vs[start : start+count] {
					free := dim.Render(bytes(v.Free) + " free")
					lines = append(lines, pair(strong.Render(volumeName(v.Mount)), metric(fmt.Sprintf("%.1f%%", v.Percent), v.Percent), w), bar(v.Percent, w), "", pair(free, dim.Render(bytes(v.Total)), w))
				}
				if count < len(vs) && len(lines) < h {
					lines = append(lines, dim.Render(fmt.Sprintf("%d–%d / %d volumes", start+1, start+count, len(vs))))
				}
				if h-len(lines) >= 3 && len(s.Disks) > 0 {
					lines = append(lines, "", dim.Render("READ / WRITE"))
					for _, d := range s.Disks {
						lines = append(lines, pair(shortName(d.Name), rate(d.Read)+" / "+rate(d.Write), w))
					}
				}
			}
		} else {
			lines = append(lines, dim.Render("No accessible mounts"))
		}
	case 4:
		vm := *m
		vm.query = "" // Detail filtering must not silently hide overview tasks.
		ps := vm.processes()
		if h < 6 {
			lines = lines[:1]
		}
		lines = append(lines, m.processHeader(w, true))
		for _, p := range ps {
			if len(lines) >= h {
				break
			}
			lines = append(lines, overviewProcessLine(p, w))
		}
		if len(ps) == 0 {
			lines = append(lines, dim.Render("No accessible processes"))
		}
	case 7, 8, 9, 10:
		return append(lines[:1], m.extraCard(m.kind(i), w, h-1)...)
	case 6:
		return append(lines[:1], m.gpuCard(w, h-1)...)
	case 5:
		if !s.RatesReady {
			return append(lines, strong.Render("Warming up…"))
		}
		down, up := traffic(s)
		if w < 32 {
			lines = append(lines, pair(dim.Render("Download"), rate(down), w), pair(dim.Render("Upload"), rate(up), w))
		} else {
			lines = append(lines, pair(lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("↓ "+rate(down)), dim.Render("↑ "+rate(up)), w))
		}
		if roomy {
			lines = append(lines, "")
		}
		extra := []string{}
		if roomy {
			for _, n := range s.Network {
				if s.Loopback[n.Name] || n.Name == "lo" || n.Name == "lo0" || strings.HasPrefix(strings.ToLower(n.Name), "loopback") {
					continue
				}
				extra = append(extra, pair(dim.Render(shortName(n.Name)), dim.Render(bytes(n.Received)+" ↓  "+bytes(n.Sent)+" ↑"), w))
				if h >= 16 {
					extra = append(extra, pair(dim.Render("Rates"), rate(n.Down)+" ↓  "+rate(n.Up)+" ↑", w), "")
				}
			}
			extra = extra[:min(len(extra), max(1, h/3))]
		}
		gh := max(1, h-len(lines)-1-len(extra))
		if !roomy {
			gh = max(1, h-len(lines))
		}
		lines = append(lines, graph(m.netHistory, w, gh, maxValue(m.netHistory), false)...)
		lines = append(lines, "")
		lines = append(lines, extra...)
	}
	if e, ok := s.Problems[m.panelTitle(i)]; ok {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(yellow).Render("! "+Clean(e)))
	}
	return lines[:min(len(lines), h)]
}
func maxValue(v []float64) float64 {
	n := 1.0
	for _, x := range v {
		if x > n {
			n = x
		}
	}
	return n
}
func (m *Model) modalRect() Rect {
	w := min(76, m.width-4)
	h := min(22, m.height-4)
	return Rect{(m.width - w) / 2, (m.height - h) / 2, w, h}
}
func (m *Model) overlay(lines []string, r Rect, color string) []*lipgloss.Layer {
	return []*lipgloss.Layer{lipgloss.NewLayer(box(lines, r.W, r.H, color)).X(r.X).Y(r.Y).Z(41)}
}
func (m *Model) helpLines() []string {
	return []string{
		strong.Render("MAKE YOURSELF AT HOME"), "",
		"↑↓←→ / hjkl           Focus dashboard cards; stop at edges",
		"↑↓ / jk               Scroll details; select process rows",
		"Click                 Select panel; inspect process row",
		"Enter                 Open selected panel",
		"g                     Open GPU panel",
		"t                     Choose this panel’s content",
		"o                     Panel manager: add, remove, order",
		"o / space             Add or remove the highlighted panel",
		"H / Delete / click -  Hide panel; restore with o / space",
		"o / shift+↑↓ / s       Move panel / sort by name",
		"o / Enter             Bring a panel to first priority",
		"f / click ↗           Pop out panel above the dashboard",
		"drag card title       Preview placement; release to commit",
		"p / arrows / Enter    Arrange with keyboard; Esc cancels",
		"Shift+arrows          Resize a card by one grid unit",
		"drag card edge        Resize width / height in grid steps",
		"s / 0 in layout       Sort titles / reset layout",
		"drag :: / any edge    Move/resize live; Esc cancels",
		"m / d                 Maximize / dock all windows",
		"1–9                   Open a panel by its original number",
		"Tab / Shift+Tab       Cycle cards or floating windows",
		"Esc                   Close the current view",
		"Space / r             Pause / refresh",
		"+ / -                 Slower / faster refresh",
		"/ / s / c             Search / sort / clear in Processes",
		"x, then y             Request process termination",
		"Mouse wheel           Scroll inside an open detail view",
		"? / q                 Help / quit", "",
		dim.Render("Top-left panels take priority. Overflow panels hide."),
		dim.Render("Esc or click to close help"),
	}
}
func cpuLabel(p metrics.Process) string {
	if !p.CPUReady {
		return "—"
	}
	return fmt.Sprintf("%.1f", p.CPU)
}
func processLine(p metrics.Process, w int) string {
	pc := dim.Render(cpuLabel(p))
	if p.CPUReady {
		pc = metric(cpuLabel(p), min(100, p.CPU))
	}
	if w < 36 {
		return fit(fmt.Sprintf("%5d ", p.PID)+strings.Repeat(" ", max(0, 5-lipgloss.Width(pc)))+pc+"  "+shortName(p.Name), w)
	}
	rss := "—"
	if p.MemoryReady {
		rss = bytes(p.RSS)
	}
	return fmt.Sprintf("%7d  ", p.PID) + pad(shortName(p.Name), w-30) + " " + strings.Repeat(" ", max(0, 7-lipgloss.Width(pc))) + pc + fmt.Sprintf("  %10s", rss)
}
func (m *Model) processTable(w, h int) []string {
	ps := m.processes()
	query := m.query
	if query == "" {
		query = "all processes"
	}
	if m.editing {
		query += "▏"
	}
	lines := []string{pair("Filter: "+Clean(query), dim.Render("sort: "+m.sortBy), w)}
	if m.processHeaderRows() == 3 {
		lines = append(lines, dim.Render(fmt.Sprintf("%d processes", len(ps))), m.processHeader(w, false))
	}
	if len(ps) == 0 {
		return append(lines, dim.Render("No matching processes. Press c to clear."))
	}
	for i := m.offset; i < len(ps) && len(lines) < h; i++ {
		p := ps[i]
		line := processLine(p, w)
		if p.PID == m.selectedPID && p.Started == m.selectedStart {
			line = lipgloss.NewStyle().Foreground(ink).Background(lipgloss.Color("#334155")).Bold(true).Render(pad(line, w))
		}
		lines = append(lines, line)
	}
	return lines
}
