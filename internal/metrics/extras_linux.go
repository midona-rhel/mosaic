//go:build linux

package metrics

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func connectionSource() string    { return "gopsutil / Linux procfs (current network namespace)" }
func readText(path string) string { b, _ := os.ReadFile(path); return strings.TrimSpace(string(b)) }
func readNumber(path string) *float64 {
	v, e := strconv.ParseFloat(readText(path), 64)
	if e != nil {
		return nil
	}
	return nonnegative(v)
}
func platformBattery(ctx context.Context) BatteryReport {
	return linuxBattery(ctx, "/sys/class/power_supply")
}
func linuxBattery(ctx context.Context, root string) BatteryReport {
	r := BatteryReport{}
	dirs, err := os.ReadDir(root)
	if err != nil {
		r.Problem = err.Error()
		return r
	}
	for _, d := range dirs {
		if ctx.Err() != nil {
			r.Problem = ctx.Err().Error()
			break
		}
		p := filepath.Join(root, d.Name())
		if readText(filepath.Join(p, "type")) != "Battery" || readText(filepath.Join(p, "present")) == "0" {
			continue
		}
		// Ignore wireless mice and other peripheral batteries.
		if readText(filepath.Join(p, "scope")) == "Device" {
			continue
		}
		n := func(k string) *float64 { return readNumber(filepath.Join(p, k)) }
		b := Battery{Name: d.Name(), State: readText(filepath.Join(p, "status")), Source: p}
		if now, full := n("energy_now"), n("energy_full"); now != nil && full != nil && *full > 0 {
			b.Remaining, b.Full, b.CapacityUnit = nonnegative(*now/1e6), nonnegative(*full/1e6), "Wh"
		} else if now, full := n("charge_now"), n("charge_full"); now != nil && full != nil && *full > 0 {
			b.Remaining, b.Full, b.CapacityUnit = nonnegative(*now/1000), nonnegative(*full/1000), "mAh"
		}
		b.Percent = n("capacity")
		if b.Percent != nil && *b.Percent > 100 {
			b.Percent = nil
		}
		if b.Percent == nil {
			b.Percent = ratio(n("energy_now"), n("energy_full"))
			if b.Percent == nil {
				b.Percent = ratio(n("charge_now"), n("charge_full"))
			}
		}
		if b.Percent != nil && *b.Percent > 100 {
			b.Percent = nil
		}
		b.Health = ratio(n("energy_full"), n("energy_full_design"))
		if b.Health == nil {
			b.Health = ratio(n("charge_full"), n("charge_full_design"))
		}
		b.Cycles = n("cycle_count")
		if v := n("voltage_now"); v != nil {
			b.Voltage = nonnegative(*v / 1e6)
		}
		if v := n("power_now"); v != nil {
			b.Watts = nonnegative(*v / 1e6)
		} else if a, e := strconv.ParseFloat(readText(filepath.Join(p, "current_now")), 64); e == nil && b.Voltage != nil {
			b.Watts = nonnegative(math.Abs(a) / 1e6 * *b.Voltage)
		}
		r.Devices = append(r.Devices, b)
	}
	return r
}
func platformFans(ctx context.Context) FanReport { return linuxFans(ctx, "/sys/class/hwmon") }
func linuxFans(ctx context.Context, root string) FanReport {
	r := FanReport{}
	files, err := filepath.Glob(filepath.Join(root, "hwmon*", "fan*_input"))
	if err != nil {
		r.Problem = err.Error()
		return r
	}
	for _, p := range files {
		if ctx.Err() != nil {
			r.Problem = ctx.Err().Error()
			break
		}
		if v := readNumber(p); v != nil && *v <= 100000 {
			base := strings.TrimSuffix(p, "_input")
			label := readText(base + "_label")
			if label == "" {
				label = filepath.Base(base)
			}
			chip := readText(filepath.Join(filepath.Dir(p), "name"))
			if chip != "" {
				label = chip + " / " + label
			}
			r.Devices = append(r.Devices, Fan{label, p, *v})
		}
	}
	if len(r.Devices) == 0 && r.Problem == "" {
		r.Problem = "No readable hwmon fan counters exposed by the kernel."
	}
	return r
}
