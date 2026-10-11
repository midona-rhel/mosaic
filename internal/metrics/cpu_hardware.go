package metrics

import (
	"context"
	"math"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

// Power domains overlap: a package includes its cores (and sometimes graphics).
// Only disjoint package sensors may contribute to the headline.
type CPUPower struct {
	ID, Name, Source string
	Package          bool
	State            string // empty for a reading, Sampling or Unavailable otherwise
	Watts            *float64
	Interval         time.Duration // zero means the provider's own sampling interval
	Limits           []CPUPowerLimit
}
type CPUPowerLimit struct {
	Name   string
	Watts  float64
	Window time.Duration
}
type CPUPowerReport struct {
	At       time.Time
	Sensors  []CPUPower
	Problems []string
}
type CPUClock struct {
	Name, Source, Kind, Governor string
	MHz                          float64
	MinMHz, MaxMHz               float64
}
type CPUHardware struct {
	PhysicalCores, LogicalCPUs int
	Clocks                     []CPUClock
	Power                      CPUPowerReport
	Problems                   []string
}
type energyCounter struct {
	Energy, Range uint64
	At            time.Time
}

func validPower(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
func (r CPUPowerReport) PackageWatts() *float64 {
	var total float64
	seen := map[string]bool{}
	for _, s := range r.Sensors {
		if !s.Package || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		if s.Watts == nil || !validPower(*s.Watts) {
			return nil
		}
		total += *s.Watts
	}
	if len(seen) == 0 || !validPower(total) {
		return nil
	}
	return &total
}

// A decrease could be wrap, reset, hotplug or a replaced device. Rebaseline
// rather than manufacture a spike by assuming a wrap. Missing samples also
// discard their baselines. Both timestamps retain Go's monotonic clock.
func energyWatts(current, previous energyCounter) (*float64, time.Duration) {
	dt := current.At.Sub(previous.At)
	if previous.At.IsZero() || dt <= 0 || current.Energy < previous.Energy || current.Range != previous.Range {
		return nil, 0
	}
	if current.Range > 0 && (current.Energy > current.Range || previous.Energy > current.Range) {
		return nil, 0
	}
	w := float64(current.Energy-previous.Energy) / 1e6 / dt.Seconds()
	if !validPower(w) {
		return nil, 0
	}
	return &w, dt
}

func (c *Collector) sampleCPUHardware(ctx context.Context) CPUHardware {
	r := platformCPUHardware(ctx, c)
	// Counts report the topology visible to this OS, not a guessed product spec.
	if c.topologyAt.IsZero() || time.Since(c.topologyAt) >= time.Minute {
		c.physicalCores, _ = cpu.CountsWithContext(ctx, false)
		c.logicalCPUs, _ = cpu.CountsWithContext(ctx, true)
		c.topologyAt = time.Now()
	}
	r.PhysicalCores, r.LogicalCPUs = c.physicalCores, c.logicalCPUs
	if r.PhysicalCores <= 0 {
		r.Problems = append(r.Problems, "Physical core count unavailable.")
	}
	if len(r.Clocks) == 0 {
		r.Problems = append(r.Problems, "Live clock readings unavailable from the OS or CPU sensor provider.")
	}
	if len(r.Power.Sensors) == 0 && len(r.Power.Problems) == 0 {
		r.Power.Problems = []string{"CPU power unavailable: no accessible package power or energy sensor."}
	}
	return r
}
