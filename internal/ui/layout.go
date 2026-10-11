package ui

import "math"

type Rect struct{ X, Y, W, H int }

func (r Rect) Contains(x, y int) bool { return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H }

type Layout struct {
	Columns, Rows, CardWidth, CardHeight int
	ColumnStep, RowStep                  int
	Cards                                []Rect
}

// Grid keeps every card identical in size. Cell rounding goes into the gutters.
// Cards grow before a breakpoint changes the column count. Overflow is hidden.
func Grid(width, height int) Layout {
	return gridWithRows(width, height, 0)
}

func gridWithRows(width, height, requestedRows int) Layout {
	usable := max(1, width-2)
	cols := 1
	if usable >= 50 {
		cols = 2
	}
	if usable >= 76 {
		cols = 3
	}
	if usable >= 110 {
		cols = 4
	}
	rows := (defaultPanelCount + cols - 1) / cols
	rows = max(rows, requestedRows)
	rows = min(rows, max(1, (height-3)/7))
	gapX, gapY := 2, 1
	cw := max(1, (usable-gapX*(cols-1))/cols)
	ch := max(1, (max(1, height-4)-gapY*(rows-1))/rows)
	extraX := usable - cw*cols - gapX*(cols-1)
	if cols > 1 {
		gapX += extraX / (cols - 1)
		extraX %= cols - 1
	}
	startX := 1 + extraX/2
	totalH := ch*rows + gapY*(rows-1)
	startY := 2 + max(0, (height-4-totalH)/2)
	l := Layout{Columns: cols, Rows: rows, CardWidth: cw, CardHeight: ch, ColumnStep: cw + gapX, RowStep: ch + gapY}
	for i := range panelCount {
		if i >= cols*rows {
			l.Cards = append(l.Cards, Rect{})
			continue
		}
		l.Cards = append(l.Cards, Rect{startX + (i%cols)*(cw+gapX), startY + (i/cols)*(ch+gapY), cw, ch})
	}
	return l
}

// Directional focus uses rectangles because a panel can span several cells.
// Only candidates overlapping the perpendicular axis count as neighbors.
func (l Layout) neighbor(slot, dx, dy int) int {
	r := l.Cards[slot]
	best, score := slot, math.MaxInt
	for i, q := range l.Cards {
		if i == slot || q.W == 0 || r.W == 0 {
			continue
		}
		gap, cross := -1, 0
		if dx != 0 && min(r.Y+r.H, q.Y+q.H) > max(r.Y, q.Y) {
			if dx > 0 {
				gap = q.X - r.X - r.W
			} else {
				gap = r.X - q.X - q.W
			}
			cross = abs(2*r.Y + r.H - 2*q.Y - q.H)
		}
		if dy != 0 && min(r.X+r.W, q.X+q.W) > max(r.X, q.X) {
			if dy > 0 {
				gap = q.Y - r.Y - r.H
			} else {
				gap = r.Y - q.Y - q.H
			}
			cross = abs(2*r.X + r.W - 2*q.X - q.W)
		}
		if gap >= 0 && gap*10000+cross < score {
			best, score = i, gap*10000+cross
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type Span struct{ W, H int }

func (m *Model) span(panel int) Span {
	s := m.spans[panel]
	if m.kind(panel) == 0 {
		return Span{min(2, max(1, s.W)), 1}
	}
	return Span{max(1, s.W), max(1, s.H)}
}

func (m *Model) panelSpanLimits(panel int) Span {
	s := m.spanLimits()
	if m.kind(panel) == 0 {
		s.W, s.H = min(s.W, 2), 1
	}
	return s
}

func (m *Model) spanLimits() Span {
	l := m.layout()
	return Span{l.Columns, l.Rows}
}

// Grid geometry depends only on the viewport, never on order or requested spans.
// Moving/resizing a card therefore cannot change everybody else's cell height.
func (m *Model) layout() Layout {
	base := Grid(m.width, m.height)
	rows := max(base.Rows, (m.height-3)/13)
	return m.pack(gridWithRows(m.width, m.height, rows))
}

func (m *Model) pack(l Layout) Layout {
	limits := Span{l.Columns, l.Rows}
	originX, originY := l.Cards[0].X, l.Cards[0].Y
	occupied := map[[2]int]bool{}
	cursor := 0
	l.Cards = make([]Rect, panelCount)
	for slot, panel := range m.order {
		if m.disabled[panel] {
			continue
		}
		s := m.span(panel)
		s.W, s.H = min(s.W, limits.W), min(s.H, limits.H)
		for cursor < l.Columns*l.Rows {
			x, y := cursor%l.Columns, cursor/l.Columns
			fits := x+s.W <= l.Columns && y+s.H <= l.Rows
			for yy := y; fits && yy < y+s.H; yy++ {
				for xx := x; xx < x+s.W; xx++ {
					if occupied[[2]int{xx, yy}] {
						fits = false
						break
					}
				}
			}
			if fits {
				for yy := y; yy < y+s.H; yy++ {
					for xx := x; xx < x+s.W; xx++ {
						occupied[[2]int{xx, yy}] = true
					}
				}
				r := Rect{originX + x*l.ColumnStep, originY + y*l.RowStep, l.CardWidth + (s.W-1)*l.ColumnStep, l.CardHeight + (s.H-1)*l.RowStep}
				l.Cards[slot] = r
				cursor++
				break
			}
			cursor++
		}
	}
	return l
}

func (m *Model) visiblePanels() []int {
	l := m.layout()
	var ids []int
	for slot, r := range l.Cards {
		if r.W > 0 {
			ids = append(ids, m.order[slot])
		}
	}
	return ids
}

func (m *Model) resizeCard(panel, dw, dh int) {
	s, limits := m.span(panel), m.panelSpanLimits(panel)
	if dw != 0 {
		s.W = min(limits.W, max(1, min(s.W, limits.W)+dw))
	}
	if dh != 0 {
		s.H = min(limits.H, max(1, min(s.H, limits.H)+dh))
	}
	m.setCardSpan(panel, s)
	m.reveal()
}

// A resized card keeps priority over panels it displaces. Never silently switch
// selection to another panel because the requested card no longer fits its row.
func (m *Model) setCardSpan(panel int, s Span) {
	if m.kind(panel) == 0 {
		s.W, s.H = min(2, max(1, s.W)), 1
	}
	m.spans[panel] = s
	for m.cardRect(panel).W == 0 && m.slot(panel) > 0 {
		m.moveCard(panel, m.slot(panel)-1)
	}
}
