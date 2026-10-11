package metrics

import (
	"github.com/shirou/gopsutil/v4/sensors"
	"strings"
)

func appleCPUThermal(values []sensors.TemperatureStat, arch string) CPUThermalReport {
	r := CPUThermalReport{}
	for _, v := range values {
		key := strings.ToLower(v.SensorKey)
		name, source := v.SensorKey, "Apple IOHID / gopsutil"
		if arch == "amd64" {
			// The pinned SMC backend returns zero for failed reads. Only its CPU diode
			// key is a die sensor; TC0H/TC0P are heatsink/proximity and stay excluded.
			if key != "tc0d" {
				continue
			}
			name, source = "CPU diode (TC0D)", "AppleSMC / gopsutil"
		} else if !(strings.HasPrefix(key, "cpu ") || strings.HasPrefix(key, "pcpu") || strings.HasPrefix(key, "ecpu") || strings.HasPrefix(key, "pacc mtr temp sensor") || strings.HasPrefix(key, "eacc mtr temp sensor")) {
			continue
		}
		if v.Temperature <= 0 || !validTemperature(v.Temperature) {
			continue
		}
		r.Sensors = append(r.Sensors, CPUTemperature{ID: v.SensorKey, Name: name, Source: source, Celsius: v.Temperature})
	}
	return r
}
