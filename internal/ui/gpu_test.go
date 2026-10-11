package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
)

func TestGPUViewsAndAvailability(t *testing.T) {
	m := New(context.Background(), Options{Demo: true})
	press(m, 'g', "g")
	if m.kind(m.selected) != 6 || !m.expanded {
		t.Fatal("GPU shortcut failed")
	}
	text := ansi.Strip(m.View().Content)
	if !strings.Contains(text, "GPU") || !strings.Contains(text, "63.0%") {
		t.Fatal("GPU values not rendered")
	}
	for _, size := range [][2]int{{80, 24}, {120, 30}, {40, 16}} {
		m.SetSize(size[0], size[1])
		assertFits(t, m)
	}
	m.snapshot.GPU = metrics.GPUReport{Devices: []metrics.GPU{{ID: "unknown", Name: "Unknown graphics", Source: "Fixture", Scope: "device"}}}
	text = ansi.Strip(strings.Join(m.gpuLines(76), "\n"))
	if !strings.Contains(text, "Unavailable") || strings.Contains(text, "0.0%") {
		t.Fatal("missing GPU value rendered as zero")
	}
}

func TestCachedGPUReportDoesNotInventHistorySamples(t *testing.T) {
	m := New(context.Background(), Options{})
	s := metrics.Demo()
	m.accept(s)
	key := s.GPU.Devices[0].Source + "/" + s.GPU.Devices[0].ID
	if len(m.gpuHistory[key]) != 1 {
		t.Fatal("GPU history sample missing")
	}
	m.accept(s)
	if len(m.gpuHistory[key]) != 1 {
		t.Fatal("cached GPU report duplicated in history")
	}
	s.GPU.At = s.GPU.At.Add(time.Second)
	m.accept(s)
	if len(m.gpuHistory[key]) != 2 {
		t.Fatal("new GPU reading was not appended")
	}
}

func TestGPUCardShowsAllocationWithoutCapacity(t *testing.T) {
	used, shared, zero := uint64(2<<30), uint64(1<<30), uint64(0)
	for _, tc := range []struct {
		name string
		gpu  metrics.GPU
		want []string
	}{
		{"WDDM", metrics.GPU{MemoryUsed: &used, SharedUsed: &shared}, []string{"Dedicated", "2.0 GiB", "Shared RAM", "1.0 GiB"}},
		{"shared only", metrics.GPU{SharedUsed: &shared}, []string{"Shared RAM", "1.0 GiB"}},
		{"valid zero", metrics.GPU{MemoryUsed: &zero}, []string{"Dedicated", "0 B"}},
		{"missing", metrics.GPU{}, []string{"GPU memory", "Unavailable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel()
			m.snapshot.GPU.Devices = []metrics.GPU{tc.gpu}
			text := ansi.Strip(strings.Join(m.gpuCard(40, 8), "\n"))
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q in %s", want, text)
				}
			}
			if strings.Contains(text, "0.0%") {
				t.Fatal("invented a percentage without capacity")
			}
		})
	}
}
