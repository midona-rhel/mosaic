package metrics

import "strings"

// Exact GPU mappings from Stats ac791a10 / Modules/Sensors/values.swift.
// A prefix scan would mix GPU, CPU and proximity sensors on different models.
func appleGPUTemperatureKeys(model string) []string {
	for _, family := range []struct{ name, keys string }{
		{"Apple M1", "Tg05 Tg0D Tg0L Tg0T"},
		{"Apple M2", "Tg0f Tg0j"},
		{"Apple M3", "Tf14 Tf18 Tf19 Tf1A Tf24 Tf28 Tf29 Tf2A"},
		{"Apple M4", "Tg0K Tg0L Tg0d Tg0e Tg0j Tg0k"},
		{"Apple M5", "Tg0U Tg0X Tg0d Tg0g Tg0j Tg1Y Tg1c Tg1g"},
	} {
		if model == family.name || strings.HasPrefix(model, family.name+" ") {
			keys := strings.Fields(family.keys)
			if family.name == "Apple M4" {
				if model == "Apple M4" {
					keys = append(keys, "Tg0G", "Tg0H")
				} else if model == "Apple M4 Pro" || model == "Apple M4 Max" {
					keys = append(keys, "Tg1U", "Tg1k")
				}
			}
			return keys
		}
	}
	return nil
}

func applyAppleGPUTemperature(g *GPU, sensors []GPUTemperatureSensor, source string) {
	g.TemperatureSensors = nil
	var sum float64
	seen := map[string]bool{}
	for _, s := range sensors {
		if seen[s.Key] || !validTemperature(s.Celsius) || s.Celsius <= 0 || s.Celsius == 128 {
			continue
		}
		seen[s.Key] = true
		g.TemperatureSensors = append(g.TemperatureSensors, s)
		sum += s.Celsius
	}
	if len(g.TemperatureSensors) > 0 {
		v := sum / float64(len(g.TemperatureSensors))
		g.Temperature = &v
		g.TemperatureSource = source
		g.TemperatureNote = "Mean of available model-matched GPU die sensors; individual readings below. Not CPU or enclosure temperature."
	}
}
