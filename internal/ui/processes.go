package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/midona-rhel/mosaic/internal/metrics"
)

func (m *Model) processes() []metrics.Process {
	ps := make([]metrics.Process, 0, len(m.snapshot.Processes))
	q := strings.ToLower(m.query)
	for _, p := range m.snapshot.Processes {
		if q == "" || strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(fmt.Sprint(p.PID), q) {
			ps = append(ps, p)
		}
	}
	sort.Slice(ps, func(i, j int) bool {
		if m.sortReverse {
			i, j = j, i
		}
		a, b := ps[i], ps[j]
		switch m.sortBy {
		case "Memory":
			if a.RSS != b.RSS {
				return a.RSS > b.RSS
			}
		case "PID":
			return a.PID < b.PID
		case "Name":
			an, bn := strings.ToLower(shortName(a.Name)), strings.ToLower(shortName(b.Name))
			if an != bn {
				return an < bn
			}
		default:
			if a.CPU != b.CPU {
				return a.CPU > b.CPU
			}
		}
		return a.PID < b.PID
	})
	return ps
}
func (m *Model) processIndex() int {
	for i, p := range m.processes() {
		if p.PID == m.selectedPID && p.Started == m.selectedStart {
			return i
		}
	}
	return 0
}
func (m *Model) chooseProcess(i int) {
	ps := m.processes()
	if len(ps) == 0 {
		m.selectedPID = 0
		m.selectedStart = 0
		if m.kind(m.selected) == 4 {
			m.offset = 0
		}
		return
	}
	p := ps[min(len(ps)-1, max(0, i))]
	m.selectedPID = p.PID
	m.selectedStart = p.Started
	if (m.expanded || m.activeWindow() >= 0) && m.kind(m.selected) == 4 {
		m.revealProcess()
	}
}
func (m *Model) ensureProcess() { m.chooseProcess(m.processIndex()) }
func (m *Model) revealProcess() {
	i := m.processIndex()
	visible := m.processVisibleRows()
	if i < m.offset {
		m.offset = i
	}
	if i >= m.offset+visible {
		m.offset = i - visible + 1
	}
	m.offset = max(0, m.offset)
}
func (m *Model) processHeaderRows() int {
	if m.processBodyHeight() <= 3 {
		return 1
	}
	return 3
}
func (m *Model) processBodyHeight() int {
	if i := m.activeWindow(); i >= 0 {
		return m.windows[i].Bounds.H - 4
	}
	return m.height - 8
}
func (m *Model) processVisibleRows() int { return max(1, m.processBodyHeight()-m.processHeaderRows()) }
func (m *Model) openProcess() tea.Cmd {
	ps := m.processes()
	if len(ps) == 0 {
		return nil
	}
	p := ps[m.processIndex()]
	m.target = &p
	m.inspection = nil
	m.inspectError = ""
	m.modalOffset = 0
	m.inspectorBounds = nil
	m.status = "Inspecting " + Clean(p.Name)
	return func() tea.Msg {
		if m.opts.Demo {
			return inspectMsg{Target: p, Detail: metrics.Detail{Process: p, User: "alex", Executable: "/Applications/" + p.Name, Command: p.Name + " --demo", Status: "running", Threads: 12, Parent: 1, ThreadsReady: true, ParentReady: true}}
		}
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		defer cancel()
		d, e := metrics.Inspect(ctx, p)
		return inspectMsg{p, d, e}
	}
}
func (m *Model) terminate() tea.Cmd {
	if m.target == nil || m.opts.ReadOnly || m.terminating {
		return nil
	}
	p := *m.target
	m.terminating = true
	return func() tea.Msg {
		if m.opts.Demo {
			return terminateMsg{p.PID, fmt.Errorf("demo mode never terminates real processes")}
		}
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		defer cancel()
		return terminateMsg{p.PID, metrics.Terminate(ctx, p)}
	}
}
