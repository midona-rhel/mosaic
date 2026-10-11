package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
	"github.com/shirou/gopsutil/v4/cpu"
)

func TestAppleSiliconRejectsImplausibleReferenceClock(t *testing.T) {
	m := testModel()
	m.snapshot.CPUInfo = []cpu.InfoStat{{ModelName: "Apple M5 Pro", Mhz: 4}}
	content := ansi.Strip(strings.Join(m.cpuHardwareDetails(100), "\n"))
	if strings.Contains(content, "4 MHz") || !strings.Contains(content, "Unavailable") {
		t.Fatal("invalid Apple Silicon descriptor presented as a frequency")
	}
	m.snapshot.CPUInfo[0].Mhz = 3504
	if !strings.Contains(ansi.Strip(strings.Join(m.cpuHardwareDetails(100), "\n")), "3504 MHz") {
		t.Fatal("valid reference frequency was suppressed")
	}
}

func TestCPUPowerAndTemperatureFitStandardViews(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 30}} {
		m := testModel()
		m.SetSize(size[0], size[1])
		for _, expanded := range []bool{false, true} {
			if expanded {
				if err := m.OpenPanel("cpu"); err != nil {
					t.Fatal(err)
				}
			}
			content := ansi.Strip(m.View().Content)
			if !strings.Contains(content, "24.6 W") || !strings.Contains(content, "64.2°C") {
				t.Fatalf("missing watts/temperature at %v expanded=%v:\n%s", size, expanded, content)
			}
			assertFits(t, m)
		}
		m.floatPanel(1)
		assertFits(t, m)
	}
	m := testModel()
	m.snapshot.CPUHardware = metrics.CPUHardware{}
	if v := ansi.Strip(m.cpuPowerLine(20)); !strings.Contains(v, "Unavailable") || strings.Contains(v, "0.0 W") {
		t.Fatal("unavailable power became zero")
	}
	m.snapshot.CPUHardware.Power.Sensors = []metrics.CPUPower{{ID: "p", Package: true, State: "Sampling"}}
	if !strings.Contains(ansi.Strip(m.cpuPowerLine(20)), "Sampling") {
		t.Fatal("missing warm-up state")
	}
	m.snapshot.CPUHardware.Power.Sensors[0].State = "Unavailable"
	if !strings.Contains(ansi.Strip(m.cpuPowerLine(20)), "Unavailable") {
		t.Fatal("unreadable power shows warm-up")
	}
}
