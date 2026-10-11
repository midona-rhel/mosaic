package metrics

import (
	"math"
	"slices"
	"testing"
)

func TestAppleGPUThermalMappingsAndAggregation(t *testing.T) {
	for model, count := range map[string]int{"Apple M1": 4, "Apple M2 Pro": 2, "Apple M3 Max": 8, "Apple M4": 8, "Apple M4 Pro": 8, "Apple M5 Pro": 8, "Apple M50": 0, "Unknown": 0} {
		keys := appleGPUTemperatureKeys(model)
		if len(keys) != count {
			t.Fatalf("%s: got %v", model, keys)
		}
		for _, key := range keys {
			if slices.Contains(appleCPUTemperatureKeys(model), key) {
				t.Fatalf("CPU sensor %s used for GPU", key)
			}
		}
	}
	if !slices.Contains(appleGPUTemperatureKeys("Apple M5 Pro"), "Tg1g") || slices.Contains(appleGPUTemperatureKeys("Apple M4 Pro"), "Tg0G") {
		t.Fatal("model-specific keys mismatched")
	}
	g := GPU{}
	applyAppleGPUTemperature(&g, []GPUTemperatureSensor{{"Tg0U", 40}, {"Tg0X", 60}, {"Tg0U", 99}, {"zero", 0}, {"bad", 128}, {"nan", math.NaN()}}, "AppleSMC")
	if g.Temperature == nil || *g.Temperature != 50 || len(g.TemperatureSensors) != 2 {
		t.Fatalf("invalid thermal mean: %+v", g)
	}
}

func TestAppleActivityCounterIdentityAndInvalidTemperature(t *testing.T) {
	data := `<plist><array><dict><key>IORegistryEntryName</key><string>AGX</string><key>PerformanceStatistics</key><dict><key>Device Utilization %</key><integer>46</integer><key>GPU Activity(%)</key><integer>18</integer><key>Temperature(C)</key><integer>128</integer></dict></dict></array></plist>`
	g, err := parseAppleGPU([]byte(data))
	if err != nil || len(g) != 1 {
		t.Fatal(err)
	}
	if g[0].Utilization == nil || *g[0].Utilization != 46 || g[0].UtilizationCounter != "Device Utilization %" {
		t.Fatal("changed raw counter units or selected an undocumented average")
	}
	if g[0].Temperature != nil {
		t.Fatal("SMC sentinel shown as temperature")
	}
}
