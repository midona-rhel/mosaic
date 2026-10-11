//go:build !linux && !darwin && !windows

package metrics

import "context"

func platformCPUHardware(context.Context, *Collector) CPUHardware {
	return CPUHardware{Power: CPUPowerReport{Problems: []string{"CPU power collection is not implemented for this OS."}}}
}
