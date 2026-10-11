//go:build darwin

package metrics

import (
	"context"
	"fmt"
	"runtime"
)

func connectionSource() string { return "gopsutil / macOS lsof (accessible processes only)" }
func platformBattery(ctx context.Context) BatteryReport {
	b, e := gpuCommand(ctx, "/usr/sbin/ioreg", "-r", "-c", "AppleSmartBattery", "-a")
	if e != nil {
		return BatteryReport{Problem: e.Error()}
	}
	r := parseAppleBattery(b)
	// PPBR is a pack-level reading, so do not assign it to multiple devices.
	if runtime.GOARCH == "arm64" && len(r.Devices) == 1 {
		smc, err := openSMC()
		if err == nil {
			defer smc.close()
			watts, readErr := smc.readNumeric(ctx, "PPBR")
			if readErr == nil {
				applyAppleBatterySMCPower(&r.Devices[0], watts)
			}
			err = readErr
		}
		if err != nil && r.Devices[0].Watts == nil {
			r.Devices[0].PowerProblem += " AppleSMC / PPBR: " + err.Error()
		}
	}
	return r
}
func platformFans(ctx context.Context) FanReport {
	r := FanReport{}
	smc, e := openSMC()
	if e != nil {
		r.Problem = e.Error()
		return r
	}
	defer smc.close()
	// Stats/iStats use FNum and F<n>Ac. No writes or fan-mode changes.
	n, e := smc.readNumeric(ctx, "FNum")
	if e != nil || n == nil || *n > 16 {
		r.Problem = "SMC fan count unavailable"
		return r
	}
	for i := 0; i < int(*n); i++ {
		key := fmt.Sprintf("F%xAc", i)
		if v, e := smc.readNumeric(ctx, key); e == nil && v != nil && *v >= 0 && *v <= 100000 {
			r.Devices = append(r.Devices, Fan{fmt.Sprintf("Fan %d", i+1), "AppleSMC / " + key, *v})
		}
	}
	if len(r.Devices) == 0 {
		r.Problem = "No fan RPM readings exposed by AppleSMC; fanless Macs have none."
	}
	return r
}
