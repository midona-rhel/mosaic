package ui

import (
	"image"
	"slices"

	"charm.land/lipgloss/v2"
)

type dragPaintEntry struct {
	width, height int
	text          string
}

// Retain panel content while moving it. Input other than motion invalidates
// this cache, including live samples, release, cancellation and terminal resize.
// Keep only the current size per panel so resizing cannot accumulate buffers.
func (m *Model) dragContent(id string, width, height int, render func() string) string {
	if m.drag == nil || m.drag.Kind == "scroll" {
		return render()
	}
	if m.dragPaint == nil {
		m.dragPaint = make(map[string]dragPaintEntry)
	}
	if old, ok := m.dragPaint[id]; ok && old.width == width && old.height == height {
		return old.text
	}
	text := render()
	m.dragPaint[id] = dragPaintEntry{width, height, text}
	return text
}

// Cache decoded cells as well as text: reparsing ANSI for every unchanged
// panel on every movement costs more than copying the cells into place.
func (m *Model) compose(canvas *lipgloss.Canvas, layers []*lipgloss.Layer) {
	if m.drag == nil || m.drag.Kind == "scroll" {
		canvas.Compose(lipgloss.NewCompositor(layers...))
		return
	}
	slices.SortStableFunc(layers, func(a, b *lipgloss.Layer) int { return a.GetZ() - b.GetZ() })
	next := make(map[string]*lipgloss.Canvas, len(layers))
	for _, layer := range layers {
		text := layer.GetContent()
		cells := m.dragCells[text]
		if cells == nil {
			cells = lipgloss.NewCanvas(lipgloss.Width(text), lipgloss.Height(text))
			cells.Compose(lipgloss.NewLayer(text))
		}
		next[text] = cells
		x, y := layer.GetX(), layer.GetY()
		cells.Draw(canvas, image.Rect(x, y, x+cells.Width(), y+cells.Height()))
	}
	m.dragCells = next
}
