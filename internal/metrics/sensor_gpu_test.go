package metrics

import (
	"github.com/midona-rhel/mosaic/internal/sensors"
	"testing"
)

func TestHelperGPUUnitsAndMissingValues(t *testing.T) {
	var rows []sensors.Sensor
	add := func(name, kind string, v *float64) {
		rows = append(rows, sensors.Sensor{ID: name, DeviceID: "/gpu-amd/0", Hardware: "AMD Radeon", HardwareType: "GpuAmd", Name: name, Type: kind, Value: v, Source: "LibreHardwareMonitorLib 0.9.6"})
	}
	add("GPU Core", "Load", sensorNumber(72))
	add("GPU Core", "Temperature", sensorNumber(61))
	add("GPU Hot Spot", "Temperature", sensorNumber(82))
	add("GPU Package", "Power", sensorNumber(0))
	add("GPU Memory Used", "SmallData", sensorNumber(1536))
	add("GPU Memory Total", "SmallData", sensorNumber(8192))
	devices := helperGPUs(sensors.Report{Sensors: rows})
	if len(devices) != 1 {
		t.Fatal(devices)
	}
	g := devices[0]
	if *g.MemoryUsed != 1536<<20 || *g.MemoryTotal != 8192<<20 || *g.Utilization != 72 || *g.Temperature != 61 || len(g.TemperatureSensors) != 2 || g.Power == nil || *g.Power != 0 {
		t.Fatal(g)
	}
	for i := range rows {
		rows[i].Value = nil
	}
	g = helperGPUs(sensors.Report{Sensors: rows})[0]
	if g.Utilization != nil || g.Temperature != nil || g.Power != nil || g.MemoryUsed != nil {
		t.Fatal("missing sensors became zeros")
	}
}
func TestHelperGPUDoesNotGuessBetweenIdenticalAdapters(t *testing.T) {
	r := GPUReport{Devices: []GPU{{ID: "first", Name: "AMD Radeon"}, {ID: "second", Name: "AMD Radeon"}}}
	mergeHelperGPUs(&r, []GPU{{ID: "helper", Name: "AMD Radeon", Temperature: sensorNumber(61)}})
	if len(r.Devices) != 3 || r.Devices[0].Temperature != nil || r.Devices[1].Temperature != nil {
		t.Fatal(r)
	}
	r = GPUReport{Devices: r.Devices[:1]}
	mergeHelperGPUs(&r, []GPU{{ID: "helper", Name: "AMD Radeon", Temperature: sensorNumber(61)}})
	if len(r.Devices) != 1 || r.Devices[0].ID != "first" || *r.Devices[0].Temperature != 61 {
		t.Fatal(r)
	}
}

// These names and units match the pinned LHM AmdGpu, NvidiaGpu,
// IntelDiscreteGpu and IntelIntegratedGpu readers. Integrated Intel load
// remains an OS engine measurement rather than an invented GPU Core value.
func TestHelperGPUVendorSensorMappings(t *testing.T) {
	for _, tc := range []struct {
		name, hardware, power string
		integrated            bool
	}{
		{"AMD Radeon", "GpuAmd", "GPU Package", false},
		{"NVIDIA GeForce", "GpuNvidia", "GPU Package", false},
		{"Intel Arc", "GpuIntel", "GPU Package", false},
		{"Intel integrated", "GpuIntel", "GPU Power", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows []sensors.Sensor
			add := func(name, kind string, value float64) {
				rows = append(rows, sensors.Sensor{ID: name, DeviceID: tc.name, Hardware: tc.name, HardwareType: tc.hardware, Name: name, Type: kind, Value: sensorNumber(value)})
			}
			add("GPU Core", "Temperature", 62)
			add(tc.power, "Power", 25)
			if tc.integrated {
				add("D3D 3D", "Load", 33)
				add("D3D Shared Memory Used", "SmallData", 256)
			} else {
				add("GPU Core", "Load", 33)
				add("GPU Memory Used", "SmallData", 256)
				add("GPU Memory Total", "SmallData", 8192)
			}
			r := GPUReport{Devices: []GPU{{ID: "os-adapter", Name: tc.name, Utilization: sensorNumber(17), UtilizationCounter: "WDDM busiest engine"}}}
			mergeHelperGPUs(&r, helperGPUs(sensors.Report{Sensors: rows}))
			if len(r.Devices) != 1 {
				t.Fatal(r)
			}
			g := r.Devices[0]
			if g.ID != "os-adapter" || g.Temperature == nil || *g.Temperature != 62 || g.Power == nil || *g.Power != 25 {
				t.Fatal(g)
			}
			if tc.integrated {
				if g.SharedUsed == nil || *g.SharedUsed != 256<<20 || g.MemoryTotal != nil || *g.Utilization != 17 || g.UtilizationCounter != "WDDM busiest engine" {
					t.Fatal(g)
				}
			} else if g.MemoryUsed == nil || *g.MemoryUsed != 256<<20 || g.MemoryTotal == nil || *g.MemoryTotal != 8192<<20 || *g.Utilization != 33 {
				t.Fatal(g)
			}
		})
	}
}
