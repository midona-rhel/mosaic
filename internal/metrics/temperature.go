package metrics

import (
	"context"
	"math"
	"time"
)

// CPU temperature is a measured sensor value, never derived from CPU load.
// Control sensors may carry vendor offsets and are not physical die readings.
type CPUTemperature struct {
	ID, Name, Source string
	Celsius          float64
	Critical         *float64
	Control          bool
}
type CPUThermalReport struct {
	At       time.Time
	Sensors  []CPUTemperature
	Problems []string
}

func validTemperature(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -50 && v <= 150
}

// The headline is the hottest measured CPU sensor. Control-only fallback is
// identified by its own label instead of silently being called die temperature.
func (r CPUThermalReport) Hottest() *CPUTemperature {
	var best *CPUTemperature
	for i := range r.Sensors {
		s := &r.Sensors[i]
		if !validTemperature(s.Celsius) {
			continue
		}
		if best == nil || (best.Control && !s.Control) || (best.Control == s.Control && s.Celsius > best.Celsius) {
			best = s
		}
	}
	return best
}
func (c *Collector) sampleCPUThermal(ctx context.Context) CPUThermalReport {
	if !c.thermal.At.IsZero() && time.Since(c.thermal.At) < 5*time.Second {
		return c.thermal
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	r := c.platformCPUThermal(ctx)
	r.At = time.Now()
	if r.Sensors == nil {
		r.Sensors = []CPUTemperature{}
	}
	if r.Problems == nil {
		r.Problems = []string{}
	}
	if len(r.Sensors) == 0 && len(r.Problems) == 0 {
		r.Problems = append(r.Problems, "No accessible CPU temperature sensor exposed by the OS or driver.")
	}
	c.thermal = r
	return r
}
