//go:build windows

package metrics

import (
	"context"
	"encoding/json"
	"time"
)

type windowsHardware struct{ Identifier, Name string }
type windowsSensor struct {
	Identifier, Parent, Name, SensorType string
	Value                                *float32
}
type windowsSensorRow struct {
	ID, Parent, Name, Hardware, Source, Type string
	Value                                    *float32
}

func windowsCPUSensors(ctx context.Context) ([]byte, error) {
	report := struct {
		Sensors  []windowsSensorRow
		Problems []string
	}{}
	for _, ns := range []string{"root/LibreHardwareMonitor", "root/OpenHardwareMonitor"} {
		hardware, err := querySensors[windowsHardware](ctx, ns, "SELECT Identifier,Name FROM Hardware WHERE HardwareType='Cpu'")
		if err != nil {
			report.Problems = append(report.Problems, ns+": "+err.Error())
			continue
		}
		sensors, err := querySensors[windowsSensor](ctx, ns, "SELECT Identifier,Parent,Name,Value,SensorType FROM Sensor WHERE SensorType='Temperature' OR SensorType='Power' OR SensorType='Clock'")
		if err != nil {
			report.Problems = append(report.Problems, ns+": "+err.Error())
			continue
		}
		for _, s := range sensors {
			for _, h := range hardware {
				if s.Parent == h.Identifier {
					report.Sensors = append(report.Sensors, windowsSensorRow{s.Identifier, s.Parent, s.Name, h.Name, ns, s.SensorType, s.Value})
				}
			}
		}
		if len(report.Sensors) > 0 {
			break
		}
	}
	return json.Marshal(report)
}

func (c *Collector) platformCPUThermal(ctx context.Context) CPUThermalReport {
	c.hardware = CPUHardware{Power: CPUPowerReport{At: time.Now(), Problems: []string{"CPU power requires an accessible WMI hardware-monitor provider with CPU Power sensors enabled."}}}
	data, err := windowsCPUSensors(ctx)
	if err != nil {
		return CPUThermalReport{Problems: []string{"CPU sensor provider: " + err.Error()}}
	}
	hardware, hardwareErr := parseWindowsCPUHardware(data)
	if hardwareErr == nil {
		c.hardware = hardware
		c.hardware.Power.At = time.Now()
	}
	r, err := parseWindowsCPUThermal(data)
	if err != nil {
		return CPUThermalReport{Problems: []string{err.Error()}}
	}
	if len(r.Sensors) == 0 {
		r.Problems = append(r.Problems, "Run a WMI-capable Libre Hardware Monitor or Open Hardware Monitor build with CPU sensors enabled. Windows ACPI thermal zones are not treated as CPU die temperature.")
	}
	return r
}
