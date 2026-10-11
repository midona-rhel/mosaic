package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
	"strings"
	"testing"
)

func TestSystemLimitsAndOSArtwork(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.resizeCard(0, 3, 3)
	if m.span(0) != (Span{2, 1}) || m.cardRect(0).H != m.layout().CardHeight {
		t.Fatal("System must stop at two columns and one row")
	}
	for _, v := range []struct{ os, platform string }{{"linux", "arch"}, {"windows", "windows"}, {"darwin", "darwin"}, {"linux", "debian"}} {
		host := *m.snapshot.Host
		host.OS, host.Platform = v.os, v.platform
		m.snapshot.Host = &host
		content := ansi.Strip(strings.Join(m.systemContent(55, 10, true), "\n"))
		art, _ := fittingSystemArt(v.os, v.platform, 55, 10, true, false)
		if len(art) == 0 || !strings.Contains(content, "Uptime") {
			t.Fatalf("missing %s art or facts: %s", v.platform, content)
		}
		for _, line := range art {
			if !strings.Contains(content, line) {
				t.Fatalf("%s artwork was clipped or omitted", v.platform)
			}
		}
	}
	m.kinds[4] = 0
	m.setCardSpan(4, Span{4, 3})
	if m.span(4) != (Span{2, 1}) {
		t.Fatal("System sizing follows panel identity instead of content")
	}
}

func TestProcessHeadingClicksSortInEveryView(t *testing.T) {
	for _, view := range []string{"card", "expanded", "floating"} {
		for _, key := range []string{"Name", "CPU", "Memory"} {
			t.Run(view+"/"+key, func(t *testing.T) {
				m := testModel()
				m.SetSize(120, 30)
				m.selected = 4
				var x, y, w int
				overview := view == "card"
				switch view {
				case "card":
					r := m.cardRect(4)
					x, y, w = r.X+2, r.Y+3, r.W-4
				case "expanded":
					m.expanded = true
					x, y, w = 3, 7, 114
				case "floating":
					m.floatPanel(4)
					r := m.windows[0].Bounds
					x, y, w = r.X+2, r.Y+5, r.W-4
				}
				for _, c := range processColumns(w, overview) {
					if c.Key != key {
						continue
					}
					m.click(mouse(x+c.Start, y))
					if m.sortBy != key || m.target != nil || m.drag != nil {
						t.Fatal("heading opened a process or failed to sort")
					}
					before := m.processes()
					reverse := m.sortReverse
					m.click(mouse(x+c.Start, y))
					after := m.processes()
					if m.sortReverse == reverse || after[0].PID != before[len(before)-1].PID {
						t.Fatal("second click did not reverse ordering")
					}
					return
				}
				t.Fatal("missing column", key)
			})
		}
	}
}

func TestDiskFilteringAndWheelPaging(t *testing.T) {
	const gb = uint64(1 << 30)
	all := []metrics.Volume{
		{Device: "devfs", Mount: "/dev", Type: "devfs", Total: 200000},
		{Device: "boot", Mount: "/boot/efi", Type: "vfat", Total: gb},
		{Device: "snap", Mount: "/", Type: "apfs", Total: 100 * gb, Free: 50 * gb},
		{Device: "data", Mount: "/System/Volumes/Data", Type: "apfs", Total: 100 * gb, Free: 50 * gb},
		{Device: "vm", Mount: "/System/Volumes/VM", Type: "apfs", Total: 100 * gb},
		{Device: "usb", Mount: "/Volumes/Small", Type: "exfat", Total: 100000000},
		{Device: "usb", Mount: "/Volumes/Alias", Type: "exfat", Total: 100000000},
	}
	got := visibleVolumes(all)
	if len(got) != 2 || got[0].Mount != "/System/Volumes/Data" || got[1].Device != "usb" {
		t.Fatalf("wrong relevant volumes: %+v", got)
	}
	m := testModel()
	m.SetSize(120, 30)
	m.snapshot.Volumes = all
	for i := 0; i < 6; i++ {
		m.snapshot.Volumes = append(m.snapshot.Volumes, metrics.Volume{Device: fmt.Sprint(i), Mount: fmt.Sprintf("/Volumes/Drive%d", i), Type: "exfat", Total: gb})
	}
	r := m.cardRect(3)
	before := m.layout()
	m.Update(tea.MouseWheelMsg{X: r.X + 3, Y: r.Y + 3, Button: tea.MouseWheelDown})
	if m.diskOffset[3] != 1 || m.cardRect(3) != before.Cards[m.slot(3)] {
		t.Fatal("wheel should change disks without scrolling dashboard")
	}
}

func TestScrollbarsTrackScrollAndRespectWindows(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	m.selected = 1
	m.expanded = true
	areas := m.scrollAreas()
	if len(areas) != 1 {
		t.Fatal("CPU detail should overflow")
	}
	a := areas[0]
	canvas := lipgloss.NewCanvas(120, 30)
	m.paintScrollbars(canvas)
	if c := canvas.CellAt(a.Rect.X, a.Rect.Y); c == nil || c.Content != "┃" {
		t.Fatal("missing scrollbar thumb")
	}
	m.click(mouse(a.Rect.X, a.Rect.Y+a.Rect.H-1))
	m.release(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: a.Rect.X, Y: a.Rect.Y + a.Rect.H - 1})
	if m.offset == 0 {
		t.Fatal("track click did not scroll")
	}
	a = m.scrollAreas()[0]
	y, _ := a.thumb()
	m.click(mouse(a.Rect.X, y))
	m.motion(tea.MouseMotionMsg{X: a.Rect.X, Y: a.Rect.Y, Button: tea.MouseLeft})
	m.release(tea.MouseReleaseMsg{X: a.Rect.X, Y: a.Rect.Y, Button: tea.MouseLeft})
	if m.offset != 0 {
		t.Fatal("dragging thumb to top did not reach top")
	}
	m.selected = 0
	if len(m.scrollAreas()) != 0 {
		t.Fatal("fitting System view has a scrollbar")
	}
	m.expanded = false
	m.floatPanel(1)
	a = m.scrollAreas()[0]
	m.floatPanel(2)
	m.windows[m.windowIndex(2)].Bounds = Rect{a.Rect.X - 3, a.Rect.Y - 1, 24, 15}
	if _, ok := m.scrollbarAt(a.Rect.X, a.Rect.Y+1); ok {
		t.Fatal("covered scrollbar caught input")
	}
}

func TestProcessScrollbarKeepsSelectionVisible(t *testing.T) {
	m := testModel()
	m.SetSize(80, 24)
	m.selected = 4
	m.expanded = true
	for i := 0; i < 80; i++ {
		m.snapshot.Processes = append(m.snapshot.Processes, metrics.Process{PID: int32(5000 + i), Name: "worker", CPU: float64(i), CPUReady: true})
	}
	a := m.scrollAreas()[0]
	m.setScrollOffset(a, 40)
	if m.offset != 40 || m.processIndex() < 40 || m.processIndex() >= 40+a.Visible {
		t.Fatal("process thumb left selection outside viewport")
	}
	m.ensureProcess()
	if m.offset != 40 {
		t.Fatal("next sample would reset scroll position")
	}
}

func TestResizeHoverKeepsBorderGlyphAndQuietFooter(t *testing.T) {
	m := testModel()
	m.SetSize(120, 30)
	r := m.cardRect(0)
	m.motion(tea.MouseMotionMsg{X: r.X, Y: r.Y})
	if m.pointerShape() != "default" {
		t.Fatal("resize cursor remains")
	}
	lines := strings.Split(ansi.Strip(m.render()), "\n")
	if []rune(lines[r.Y])[r.X] != '╭' {
		t.Fatal("corner replaced by an arrow")
	}
	if strings.Contains(ansi.Strip(m.footer()), "Drag edge") {
		t.Fatal("hover instructions remain")
	}
}

func TestOSCatalogueCoverageAndBounds(t *testing.T) {
	for alias, name := range artCatalogue.Aliases {
		if artCatalogue.Art[name] == "" {
			t.Fatalf("missing artwork for %s", alias)
		}
	}
	cases := map[string]string{
		"Arch Linux": "arch", "ubuntu": "ubuntu", "linuxmint": "linuxmint",
		"debian": "debian", "fedora": "fedora", "rhel": "rhel", "Pop!_OS": "pop",
		"openSUSE Tumbleweed": "opensuse_tumbleweed", "nixos": "nixos", "alpine": "alpine",
		"gentoo": "gentoo", "manjaro": "manjaro", "amzn": "amazon_linux",
		"unlisted-distribution": "linux",
	}
	for platform, want := range cases {
		if got := systemArtName("linux", platform); got != want {
			t.Errorf("%s: %s, want %s", platform, got, want)
		}
	}
	for _, platform := range []string{"windows", "arch", "ubuntu", "fedora", "linuxmint", "darwin"} {
		for _, size := range [][2]int{{120, 30}, {80, 24}, {55, 20}, {24, 10}} {
			m := testModel()
			host := *m.snapshot.Host
			host.Platform, host.OS = platform, "linux"
			if platform == "windows" || platform == "darwin" {
				host.OS = platform
			}
			m.snapshot.Host, m.expanded = &host, true
			m.SetSize(size[0], size[1])
			r := m.expandedRect()
			if r.X < 1 || r.Y < 2 || r.X+r.W > m.width-1 || r.Y+r.H > m.height-2 {
				t.Fatalf("%s has offscreen details: %+v", platform, r)
			}
			for _, a := range m.expandedActions() {
				if !r.Contains(a.Rect.X, a.Rect.Y) {
					t.Fatal("detached detail action")
				}
			}
			if size[0] == 120 {
				art, _ := fittingSystemArt(host.OS, platform, r.W-4, r.H-4, true, false)
				if len(art) == 0 {
					t.Fatalf("%s reference artwork missing at 120x30", platform)
				}
			}
			for _, line := range strings.Split(m.render(), "\n") {
				if ansi.StringWidth(line) > m.width {
					t.Fatal("render overflow")
				}
			}
		}
	}
}

func TestCompactLogosReserveOneRow(t *testing.T) {
	for _, height := range []int{7, 8, 9} {
		for _, detail := range []bool{false, true} {
			art, _ := fittingSystemArt("darwin", "macOS", 52, height, detail, false)
			joined := strings.Join(art, "\n")
			if len(art) != height-1 || artWidth(art) > 23 || !strings.Contains(joined, "@@@") || strings.Contains(joined, "____") {
				t.Fatalf("compact %d-row art is missing, oversized or still outlined: %s", height, joined)
			}
		}
	}
	art, _ := fittingSystemArt("darwin", "macOS", 90, 22, true, false)
	if !strings.Contains(strings.Join(art, "\n"), "MMMM") {
		t.Fatal("full reference was replaced")
	}
	for _, height := range []int{26, 28, 30} {
		m := testModel()
		m.SetSize(120, height)
		m.resizeCard(0, 1, 0)
		r := m.cardRect(0)
		body := strings.Join(m.cardLines(0, r.W-4, r.H-2), "\n")
		if !strings.Contains(body, "@@@") || strings.Contains(body, "____") {
			t.Fatalf("2x1 card still uses outline at %d rows", height)
		}
	}
}

func TestCompactLogoStylesAndDimensions(t *testing.T) {
	for name, variants := range compactOSArt {
		for _, art := range variants {
			if len(art) == 0 || len(art) > 8 || artWidth(art) > 23 || !plainASCIIArt(art) {
				t.Fatalf("%s: invalid compact ASCII reference", name)
			}
		}
		for _, height := range []int{7, 8, 9, 12} {
			for _, asciiOnly := range []bool{false, true} {
				art, _ := fittingSystemArt("linux", name, 52, height, false, asciiOnly)
				if len(art) > 8 || len(art) >= height || artWidth(art) > 23 || !plainASCIIArt(art) {
					t.Fatalf("%s: artwork does not fit or contains non-ASCII cells", name)
				}
				if height >= 9 && len(art) == 0 {
					t.Fatalf("%s: reference is missing at standard card height", name)
				}
			}
		}
	}
}
