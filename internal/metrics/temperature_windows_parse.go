package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseWindowsCPUThermal(data []byte) (CPUThermalReport, error) {
	var raw struct {
		Sensors []struct {
			ID, Name, Hardware, Source, Type string
			Value                            *float64
		}
		Problems []string
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CPUThermalReport{}, fmt.Errorf("invalid CPU sensor report: %w", err)
	}
	r := CPUThermalReport{Problems: raw.Problems}
	for _, s := range raw.Sensors {
		// These WMI providers publish missing values as zero. Do not display those.
		if (s.Type != "" && s.Type != "Temperature") || s.Value == nil || *s.Value <= 0 || !validTemperature(*s.Value) || s.ID == "" {
			continue
		}
		r.Sensors = append(r.Sensors, CPUTemperature{ID: s.ID, Name: s.Hardware + " / " + s.Name, Source: s.Source, Celsius: *s.Value, Control: strings.Contains(strings.ToLower(s.Name), "tctl")})
	}
	if len(r.Sensors) > 0 {
		r.Problems = nil
	}
	return r, nil
}
