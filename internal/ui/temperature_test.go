package ui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
	"strings"
	"testing"
)

func TestCPUTemperatureVisibleAtBothStandardSizesAndInPopout(t *testing.T) {
	m := testModel()
	for _, size := range [][2]int{{80, 24}, {120, 30}} {
		m.SetSize(size[0], size[1])
		m.expanded = false
		if !strings.Contains(ansi.Strip(m.View().Content), "64.2°C") {
			t.Fatal("CPU card lost temperature")
		}
		if err := m.OpenPanel("cpu"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ansi.Strip(m.View().Content), "64.2°C") {
			t.Fatal("CPU detail lost temperature")
		}
		assertFits(t, m)
	}
	m.floatPanel(1)
	if !strings.Contains(ansi.Strip(m.View().Content), "64.2°C") {
		t.Fatal("CPU popout lost temperature")
	}
	m.snapshot.CPUThermal = metrics.CPUThermalReport{Problems: []string{"Sensor permission denied"}}
	if line := ansi.Strip(m.cpuTemperatureLine(24)); !strings.Contains(line, "Unavailable") || strings.Contains(line, "0.0°C") {
		t.Fatal("failed sensor was hidden or displayed as zero")
	}
	if !strings.Contains(ansi.Strip(strings.Join(m.cpuTemperatureDetails(74), "\n")), "Sensor permission denied") {
		t.Fatal("missing reason for unavailable data")
	}
	m.snapshot.CPUThermal.Sensors = []metrics.CPUTemperature{{Name: "Tctl", Celsius: 85, Control: true}}
	if !strings.Contains(ansi.Strip(m.cpuTemperatureLine(24)), "Tctl control") {
		t.Fatal("control reading mislabeled as physical temperature")
	}
}
