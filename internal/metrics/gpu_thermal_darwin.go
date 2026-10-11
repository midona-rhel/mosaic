//go:build darwin

package metrics

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

func (c *Collector) enrichGPU(ctx context.Context, r *GPUReport) {
	if runtime.GOARCH != "arm64" {
		return
	}
	var indexes []int
	for i, g := range r.Devices {
		if strings.Contains(strings.ToLower(g.Name), "agx") {
			indexes = append(indexes, i)
		}
	}
	// A package sensor cannot safely be assigned to an ambiguous device list.
	if len(indexes) != 1 {
		return
	}
	g := &r.Devices[indexes[0]]
	if c.cpuModel != "" {
		g.Note += " Registry device: " + g.Name + "."
		g.Name = c.cpuModel + " GPU"
	}
	if g.Temperature != nil {
		return
	}
	keys := appleGPUTemperatureKeys(c.cpuModel)
	if len(keys) == 0 {
		g.TemperatureNote = "No verified AppleSMC GPU sensor mapping for " + c.cpuModel
		return
	}
	s, err := openSMC()
	if err != nil {
		g.TemperatureNote = err.Error()
		return
	}
	defer s.close()
	var sensors []GPUTemperatureSensor
	for _, key := range keys {
		v, err := s.read(ctx, key)
		if err == nil && v != nil && validTemperature(*v) && *v > 0 && *v != 128 {
			sensors = append(sensors, GPUTemperatureSensor{key, *v})
		}
		if ctx.Err() != nil {
			break
		}
	}
	applyAppleGPUTemperature(g, sensors, "AppleSMC / "+s.endpoint)
	if g.Temperature == nil {
		g.TemperatureNote = fmt.Sprintf("None of %d model-matched GPU temperature keys returned a valid reading on %s.", len(keys), c.cpuModel)
	}
}
