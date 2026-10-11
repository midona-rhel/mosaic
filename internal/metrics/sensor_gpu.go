package metrics

import (
	"fmt"
	"math"
	"strings"

	"github.com/midona-rhel/mosaic/internal/sensors"
)

func helperGPUs(report sensors.Report) []GPU {
	var result []GPU
	indices := map[string]int{}
	for _, s := range report.Sensors {
		if !strings.HasPrefix(s.HardwareType, "Gpu") || s.DeviceID == "" {
			continue
		}
		index, ok := indices[s.DeviceID]
		if !ok {
			index = len(result)
			indices[s.DeviceID] = index
			vendor := strings.TrimPrefix(s.HardwareType, "Gpu")
			result = append(result, GPU{ID: s.DeviceID, Name: s.Hardware, Vendor: vendor, Source: s.Source, Scope: "device", Note: "Direct hardware-library sensors; GPU Core load uses the vendor provider where available."})
		}
		if s.Value == nil || math.IsNaN(*s.Value) || math.IsInf(*s.Value, 0) {
			continue
		}
		g, v := &result[index], *s.Value
		switch s.Type {
		case "Load":
			if s.Name == "GPU Core" && v >= 0 && v <= 100 {
				g.Utilization = sensorNumber(v)
				g.UtilizationCounter = "LibreHardwareMonitor / GPU Core"
			}
		case "Temperature":
			if validTemperature(v) {
				g.TemperatureSensors = append(g.TemperatureSensors, GPUTemperatureSensor{Key: s.Name, Celsius: v})
				if s.Name == "GPU Core" {
					g.Temperature = sensorNumber(v)
					g.TemperatureSource = s.Source + " / " + s.Name
				}
			}
		case "Power":
			if (s.Name == "GPU Package" || s.Name == "GPU Power") && validPower(v) {
				g.Power = sensorNumber(v)
			}
		case "SmallData":
			if v < 0 || v >= float64(^uint64(0))/(1<<20) {
				continue
			}
			bytes := uint64(v * (1 << 20))
			switch s.Name {
			case "GPU Memory Used":
				g.MemoryUsed = &bytes
			case "GPU Memory Total":
				g.MemoryTotal = &bytes
			case "D3D Dedicated Memory Used":
				if g.MemoryUsed == nil {
					g.MemoryUsed = &bytes
				}
			case "D3D Dedicated Memory Total":
				if g.MemoryTotal == nil {
					g.MemoryTotal = &bytes
				}
			case "D3D Shared Memory Used":
				g.SharedUsed = &bytes
			}
		}
	}
	for i := range result {
		g := &result[i]
		if g.MemoryUsed != nil && g.MemoryTotal != nil && *g.MemoryUsed > *g.MemoryTotal {
			g.MemoryUsed = nil
		}
	}
	return result
}

// Merge only one-to-one names. Two identical GPUs must never receive each
// other's temperatures or memory readings based on enumeration order.
func mergeHelperGPUs(r *GPUReport, helper []GPU) {
	key := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	for _, g := range helper {
		matches, index, helperMatches := 0, -1, 0
		for i, old := range r.Devices {
			if key(old.Name) == key(g.Name) {
				matches++
				index = i
			}
		}
		for _, other := range helper {
			if key(other.Name) == key(g.Name) {
				helperMatches++
			}
		}
		if matches == 1 && helperMatches == 1 {
			old := r.Devices[index]
			if g.Utilization == nil {
				g.Utilization = old.Utilization
				g.UtilizationCounter = old.UtilizationCounter
			}
			if g.MemoryUsed == nil {
				g.MemoryUsed = old.MemoryUsed
			}
			if g.MemoryTotal == nil {
				g.MemoryTotal = old.MemoryTotal
			}
			if g.SharedUsed == nil {
				g.SharedUsed = old.SharedUsed
			}
			if g.Temperature == nil {
				g.Temperature = old.Temperature
				g.TemperatureSource = old.TemperatureSource
			}
			if g.Power == nil {
				g.Power = old.Power
			}
			g.ID = old.ID
			g.PCI = old.PCI
			g.Source = old.Source + " + " + g.Source
			r.Devices[index] = g
		} else {
			if matches > 0 {
				g.Note += fmt.Sprintf(" Identity is ambiguous across %d OS entries; shown separately.", matches)
			}
			r.Devices = append(r.Devices, g)
		}
	}
}

func sensorNumber(v float64) *float64 { return &v }
