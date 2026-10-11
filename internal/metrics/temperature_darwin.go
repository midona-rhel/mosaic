//go:build darwin

package metrics

import (
	"context"
	"github.com/shirou/gopsutil/v4/sensors"
	"runtime"
)

func (c *Collector) platformCPUThermal(ctx context.Context) CPUThermalReport {
	var smc CPUThermalReport
	if runtime.GOARCH == "arm64" {
		smc = readSMCThermal(ctx, c.cpuModel)
		if len(smc.Sensors) > 0 {
			return smc
		}
	}
	values, err := sensors.TemperaturesWithContext(ctx)
	r := appleCPUThermal(values, runtime.GOARCH)
	if err != nil {
		r.Problems = append(r.Problems, err.Error())
	}
	if len(r.Sensors) == 0 {
		r.Problems = append(r.Problems, smc.Problems...)
		r.Problems = append(r.Problems, "macOS did not expose recognized CPU sensors through AppleSMC / IOHID. Availability depends on the Mac model and macOS version.")
	}
	return r
}
