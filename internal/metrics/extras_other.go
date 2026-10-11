//go:build !linux && !windows && !darwin

package metrics

import "context"

func connectionSource() string { return "gopsutil / OS socket tables" }
func platformBattery(context.Context) BatteryReport {
	return BatteryReport{Problem: "Battery provider unavailable on this operating system"}
}
func platformFans(context.Context) FanReport {
	return FanReport{Problem: "Fan provider unavailable on this operating system"}
}
