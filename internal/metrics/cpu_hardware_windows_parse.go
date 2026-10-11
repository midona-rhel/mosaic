package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseWindowsCPUHardware(data []byte) (CPUHardware, error) {
	var raw struct {
		Sensors []struct {
			ID, Name, Hardware, Source, Type, Parent string
			Value                                    *float64
		}
		Problems []string
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CPUHardware{}, fmt.Errorf("invalid CPU hardware report: %w", err)
	}
	r := CPUHardware{Power: CPUPowerReport{Problems: raw.Problems}}
	packages := map[string]bool{}
	seen := map[string]bool{}
	for _, s := range raw.Sensors {
		if s.ID == "" || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		switch s.Type {
		case "Power":
			// LHM/OHM publish a missing float as zero. A WMI zero is therefore
			// ambiguous; preserve it as unavailable instead of claiming 0 W.
			var watts *float64
			if s.Value != nil && validPower(*s.Value) && *s.Value > 0 {
				watts = s.Value
			}
			name := strings.ToLower(strings.TrimSpace(s.Name))
			pkg := (name == "cpu package" || name == "package") && s.Parent != "" && !packages[s.Parent]
			if pkg {
				packages[s.Parent] = true
			}
			r.Power.Sensors = append(r.Power.Sensors, CPUPower{ID: s.ID, Name: s.Hardware + " / " + s.Name, Source: s.Source, Watts: watts, Package: pkg})
		case "Clock":
			if s.Value != nil && validPower(*s.Value) && *s.Value > 0 && strings.Contains(strings.ToLower(s.Name), "core") {
				r.Clocks = append(r.Clocks, CPUClock{Name: s.Hardware + " / " + s.Name, Source: s.Source, Kind: "Provider reported", MHz: *s.Value})
			}
		}
	}
	if len(r.Power.Sensors) > 0 {
		r.Power.Problems = nil
	} else {
		r.Power.Problems = append(r.Power.Problems, "No CPU Power sensors returned. Run a WMI-capable Libre Hardware Monitor or Open Hardware Monitor build with CPU sensors enabled.")
	}
	return r, nil
}
