//go:build !linux && !darwin && !windows

package metrics

import "context"

func (c *Collector) platformCPUThermal(context.Context) CPUThermalReport {
	return CPUThermalReport{Problems: []string{"CPU temperature is unsupported on this OS."}}
}
