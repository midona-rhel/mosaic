package ui

import (
	"context"
	"fmt"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/midona-rhel/mosaic/internal/metrics"
)

const panelCount = 11
const defaultPanelCount = 8

var panels = []string{"System", "CPU", "Memory", "Disks", "Processes", "Network", "GPU", "Disk activity", "Battery", "Thermals", "Connections"}

type Options struct {
	Interval                time.Duration
	Demo, NoMouse, ReadOnly bool
	NoTerminalEffects       bool
	ASCIIArt                bool
}
type sampleMsg metrics.Snapshot
type tickMsg struct{ Generation int }
type inspectMsg struct {
	Target metrics.Process
	Detail metrics.Detail
	Err    error
}
type terminateMsg struct {
	PID int32
	Err error
}

type Model struct {
	meterPreference                                             string
	ctx                                                         context.Context
	collector                                                   *metrics.Collector
	opts                                                        Options
	width, height, selected, offset                             int
	expanded, paused, help, busy, editing, confirm, terminating bool
	picking, refreshOnce                                        bool
	pickerIndex                                                 int
	organizing                                                  bool
	organizerIndex                                              int
	kinds                                                       [panelCount]int
	disabled                                                    [panelCount]bool
	diskHistory                                                 map[string][2][]float64
	query, sortBy, status                                       string
	sortReverse                                                 bool
	diskOffset                                                  [panelCount]int
	snapshot                                                    metrics.Snapshot
	cpuHistory, memHistory, netHistory                          []float64
	gpuHistory                                                  map[string][]float64
	selectedPID                                                 int32
	selectedStart                                               int64
	target                                                      *metrics.Process
	inspection                                                  *metrics.Detail
	inspectError                                                string
	modalOffset                                                 int
	helpOffset, tickGeneration                                  int
	windows                                                     []floatingWindow
	focusWindow                                                 int
	drag                                                        *gesture
	mouseX, mouseY                                              int
	inspectorBounds                                             *Rect
	order                                                       [panelCount]int
	reordering                                                  bool
	reorderOrigin                                               [panelCount]int
	reorderSelected                                             int
	spans, reorderSpans                                         [panelCount]Span
}

func New(ctx context.Context, opts Options) *Model {
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}
	m := &Model{ctx: ctx, collector: metrics.NewCollector(), opts: opts, width: 80, height: 24, sortBy: "CPU", status: "Collecting system data…", focusWindow: -1, mouseX: -1, mouseY: -1}
	m.order = defaultOrder()
	for i := range m.spans {
		m.spans[i] = Span{1, 1}
		m.kinds[i] = i
		m.disabled[i] = i >= defaultPanelCount
	}
	if opts.Demo {
		m.snapshot = metrics.Demo()
		m.status = ""
		m.cpuHistory = nil
		m.memHistory = nil
		m.netHistory = nil
		m.diskHistory = map[string][2][]float64{}
		for i := range historyLimit {
			x := float64(i)
			m.cpuHistory = append(m.cpuHistory, 35+22*math.Sin(x*.14)+10*math.Cos(x*.43))
			m.memHistory = append(m.memHistory, 40+6*math.Sin(x*.07))
			m.netHistory = append(m.netHistory, 500000+220000*math.Sin(x*.19)+110000*math.Cos(x*.61))
			for _, d := range m.snapshot.Disks {
				v := m.diskHistory[d.Name]
				v[0] = append(v[0], d.Read*(1+.4*math.Sin(x*.15)))
				v[1] = append(v[1], d.Write*(1+.4*math.Sin(x*.3)))
				m.diskHistory[d.Name] = v
			}
		}
		m.gpuHistory = map[string][]float64{}
		for _, g := range m.snapshot.GPU.Devices {
			if g.Utilization != nil {
				for i := range historyLimit {
					m.gpuHistory[g.Source+"/"+g.ID] = append(m.gpuHistory[g.Source+"/"+g.ID], 45+25*math.Sin(float64(i)*.08))
				}
			}
		}
	}
	return m
}
func (m *Model) SetSize(w, h int) {
	m.width = max(1, w)
	m.height = max(1, h)
	m.drag = nil
	for i := range m.windows {
		m.windows[i].Bounds = clampWindow(m.windows[i].Bounds, m.width, m.height)
	}
	if m.inspectorBounds != nil {
		r := clampWindow(*m.inspectorBounds, m.width, m.height)
		m.inspectorBounds = &r
	}
	m.reveal()
	if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) == 4 {
		m.revealProcess()
	}
	m.clampOffsets()
}
func (m *Model) SetSnapshot(s metrics.Snapshot) { m.accept(s) }
func (m *Model) Init() tea.Cmd                  { return m.sample() }
func (m *Model) sample() tea.Cmd {
	if m.busy {
		return nil
	}
	m.tickGeneration++ // Invalidate a scheduled tick when manually refreshing.
	m.busy = true
	wants := metrics.ExtraOptions{}
	for p := range m.kinds {
		shown := !m.disabled[p] || (m.expanded && m.selected == p) || m.windowIndex(p) >= 0
		if !shown {
			continue
		}
		switch m.kind(p) {
		case 8:
			wants.Battery = true
		case 9:
			wants.Fans = true
		case 10:
			wants.Connections = true
		}
	}
	return func() tea.Msg {
		if m.opts.Demo {
			return sampleMsg(metrics.Demo())
		}
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		defer cancel()
		return sampleMsg(m.collector.SampleWithExtras(ctx, wants))
	}
}
func (m *Model) tick() tea.Cmd {
	m.tickGeneration++
	generation := m.tickGeneration
	return tea.Tick(m.opts.Interval, func(time.Time) tea.Msg { return tickMsg{generation} })
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(v.Width, v.Height)
	case sampleMsg:
		m.busy = false
		if !m.paused || m.refreshOnce {
			m.accept(metrics.Snapshot(v))
		}
		m.refreshOnce = false
		if !m.paused {
			return m, m.tick()
		}
	case tickMsg:
		if !m.paused && v.Generation == m.tickGeneration {
			return m, m.sample()
		}
	case inspectMsg:
		if m.target != nil && m.target.PID == v.Target.PID && m.target.Started == v.Target.Started {
			if v.Err != nil {
				m.inspectError = Clean(v.Err.Error())
			} else {
				m.inspection = &v.Detail
			}
		}
	case terminateMsg:
		m.terminating = false
		m.confirm = false
		m.target = nil
		m.inspection = nil
		if v.Err != nil {
			m.status = "Terminate failed: " + Clean(v.Err.Error())
		} else {
			m.status = fmt.Sprintf("Termination requested for PID %d", v.PID)
		}
		if !m.paused {
			return m, m.sample()
		}
	case tea.KeyPressMsg:
		return m, m.key(v)
	case tea.PasteMsg:
		if m.editing {
			m.query = fit(Clean(m.query+v.Content), 128)
			m.ensureProcess()
		}
	case tea.MouseClickMsg:
		return m, m.click(v)
	case tea.MouseMotionMsg:
		m.motion(v)
	case tea.MouseReleaseMsg:
		m.release(v)
	case tea.MouseWheelMsg:
		if m.opts.NoMouse || m.confirm || m.picking || m.organizing || m.drag != nil {
			return m, nil
		}
		if v.Button != tea.MouseWheelUp && v.Button != tea.MouseWheelDown {
			return m, nil
		}
		d := 3
		if v.Button == tea.MouseWheelUp {
			d = -3
		}
		if m.help {
			m.helpOffset = min(max(0, m.helpOffset+d), max(0, len(m.helpLines())-(m.modalRect().H-2)))
			return m, nil
		}
		if m.target == nil && !m.expanded && len(m.windows) > 0 {
			i := -1
			for j := len(m.windows) - 1; j >= 0; j-- {
				if m.windows[j].Bounds.Contains(v.X, v.Y) {
					i = j
					break
				}
			}
			if i >= 0 {
				m.raise(i)
			} else {
				if old := m.activeWindow(); old >= 0 {
					m.windows[old].Offset = m.offset
				}
				m.focusWindow = -1
				m.offset = 0
			}
		}
		if m.target != nil {
			m.modalOffset += d
			m.clampOffsets()
		} else if m.expanded || m.activeWindow() >= 0 {
			m.moveDetail(d)
		} else {
			for _, p := range m.order {
				r := m.cardRect(p)
				if m.kind(p) == 3 && r.Contains(v.X, v.Y) {
					step := 1
					if d < 0 {
						step = -1
					}
					m.diskOffset[p] = min(max(0, m.diskOffset[p]+step), max(0, len(visibleVolumes(m.snapshot.Volumes))-diskCardCount(r.H-2)))
					break
				}
			}
		}
	}
	return m, nil
}
func (m *Model) reveal() {
	if !m.expanded && m.activeWindow() < 0 && m.cardRect(m.selected).W == 0 {
		if ids := m.visiblePanels(); len(ids) > 0 {
			m.selected = ids[0]
		}
	}
}
func (m *Model) moveDetail(d int) {
	if m.kind(m.selected) == 4 {
		m.chooseProcess(m.processIndex() + d)
	} else {
		m.offset += d
		m.clampOffsets()
	}
}
func (m *Model) clampOffsets() {
	m.modalOffset = max(0, m.modalOffset)
	if m.target != nil {
		r := m.inspectorRect()
		m.modalOffset = min(m.modalOffset, max(0, len(m.processDetailLines(r.W-4))-(r.H-6)))
	}
	if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) != 4 {
		r := m.expandedRect()
		w, h := r.W-4, r.H-4
		if i := m.activeWindow(); i >= 0 {
			w, h = m.windows[i].Bounds.W-4, m.windows[i].Bounds.H-4
		}
		m.offset = min(max(0, m.offset), max(0, len(m.detailLines(max(1, w)))-max(1, h)))
	}
	if i := m.activeWindow(); i >= 0 {
		m.windows[i].Offset = m.offset
	}
}
func defaultOrder() (order [panelCount]int) {
	for i := range order {
		order[i] = i
	}
	return
}
