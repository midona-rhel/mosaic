package ui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Fastfetch assets and aliases are preserved in this catalogue; see osart/LICENSE.
//
//go:embed osart/catalog.json
var osArtJSON []byte

// Compact ASCII references and adaptations; attribution is in osart/README.md.
//
//go:embed osart/compact.json
var compactOSJSON []byte

var compactOSArt = func() map[string][][]string {
	var variants map[string][][]string
	if err := json.Unmarshal(compactOSJSON, &variants); err != nil {
		panic(err)
	}
	return variants
}()

var artColorMarker = regexp.MustCompile(`\$[0-9]+`)

type osArtCatalog struct {
	Art     map[string]string
	Aliases map[string]string
}

var artCatalogue = func() osArtCatalog {
	var catalog osArtCatalog
	if err := json.Unmarshal(osArtJSON, &catalog); err != nil {
		panic(err)
	}
	return catalog
}()

func normalizeOS(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func systemArtName(osName, platform string) string {
	if osName == "darwin" {
		return "macos"
	}
	if osName == "windows" {
		return "windows_11"
	}
	// Names returned by OS APIs that differ from Fastfetch's aliases.
	key := normalizeOS(platform)
	switch key {
	case "archlinux":
		key = "arch"
	case "mint":
		key = "linuxmint"
	}
	if name, ok := artCatalogue.Aliases[key]; ok {
		return name
	}
	if name, ok := artCatalogue.Aliases[normalizeOS(osName)]; ok {
		return name
	}
	return "linux"
}

func systemArt(osName, platform string, compact bool) ([]string, string) {
	name := systemArtName(osName, platform)
	if compact {
		if _, ok := artCatalogue.Art[name+"_small"]; ok {
			name += "_small"
		}
	}
	raw := artCatalogue.Art[name]
	return strings.Split(strings.TrimRight(artColorMarker.ReplaceAllString(raw, ""), "\n"), "\n"), "#538FFF"
}

func artWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line))
	}
	return width
}

func plainASCIIArt(lines []string) bool {
	for _, line := range lines {
		for _, char := range line {
			if char < ' ' || char > '~' {
				return false
			}
		}
	}
	return true
}

// Compact artwork leaves a row of breathing room below the mark. Full detail
// views keep the larger reference when it fits.
func fittingSystemArt(osName, platform string, w, h int, detailed, asciiOnly bool) ([]string, string) {
	name := systemArtName(osName, platform)
	if detailed {
		art, color := systemArt(osName, platform, false)
		if (!asciiOnly || plainASCIIArt(art)) && len(art) <= h && artWidth(art)+3+26 <= w {
			return art, color
		}
	}
	maxH := min(8, h-1)
	if variants, ok := compactOSArt[name]; ok {
		for _, art := range variants {
			if len(art) <= maxH && artWidth(art)+3+26 <= w {
				return art, "#538FFF"
			}
		}
		return nil, "#538FFF"
	}
	art, color := systemArt(osName, platform, true)
	if plainASCIIArt(art) && len(art) <= maxH && artWidth(art)+3+26 <= w {
		return art, color
	}
	return nil, "#538FFF"
}

func (m *Model) expandedRect() Rect {
	r := Rect{1, 2, m.width - 2, m.height - 4}
	if m.kind(m.selected) == 0 {
		art := []string(nil)
		if v := m.snapshot.Host; v != nil {
			art, _ = fittingSystemArt(v.OS, v.Platform, r.W-4, r.H-4, true, m.opts.ASCIIArt)
		}
		r.W = min(max(62, artWidth(art)+3+38+4), r.W)
		r.H = min(max(13, len(art)+6), r.H)
		r.X, r.Y = (m.width-r.W)/2, (m.height-r.H)/2
	}
	return r
}

func (m *Model) systemContent(w, h int, detailed bool) []string {
	v := m.snapshot.Host
	if v == nil {
		return []string{dim.Render("System unavailable")}
	}
	var art []string
	color := "#538FFF"
	artWidth := 0
	if w >= 42 {
		art, color = fittingSystemArt(v.OS, v.Platform, w, h, detailed, m.opts.ASCIIArt)
		artWidth = 0
		for _, line := range art {
			artWidth = max(artWidth, ansi.StringWidth(line))
		}
	}

	iw := w
	if artWidth > 0 {
		iw -= artWidth + 3
	}
	name := v.Platform
	if v.OS == "darwin" {
		name = "macOS"
	}
	info := []string{strong.Render(fit(Clean(name+" "+v.PlatformVersion), iw))}
	add := func(k, value string) { info = append(info, pair(dim.Render(k), Clean(value), iw)) }
	add("Host", v.Hostname)
	add("Architecture", v.KernelArch)
	add("Uptime", duration(v.Uptime))
	add("Kernel", v.KernelVersion)
	add("Processes", fmt.Sprint(v.Procs))
	if detailed {
		add("Booted", time.Unix(int64(v.BootTime), 0).Local().Format("02 Jan 15:04"))
	}
	if v.VirtualizationSystem != "" {
		add("Virtualization", v.VirtualizationSystem+" "+v.VirtualizationRole)
	}
	if detailed {
		h = max(h, len(info))
	}
	if artWidth == 0 {
		return info[:min(max(0, h), len(info))]
	}
	var lines []string
	infoTop := max(0, (len(art)-len(info))/2)
	for i := 0; i < min(h, max(len(info), len(art))); i++ {
		left, right := "", ""
		if i < len(art) {
			colors := []string{"#538FFF", "#6285FF", "#7977FF", "#9467FF", "#AC5EFF", "#C65AE6"}
			color = colors[min(len(colors)-1, i*len(colors)/len(art))]
			left = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(art[i])
		}
		if j := i - infoTop; j >= 0 && j < len(info) {
			right = info[j]
		}
		lines = append(lines, pad(left, artWidth)+"   "+right)
	}
	return lines
}
