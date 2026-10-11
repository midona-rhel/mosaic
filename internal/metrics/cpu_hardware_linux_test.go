//go:build linux

package metrics

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLinuxRAPLHierarchyAliasesLimitsAndDisappearance(t *testing.T) {
	root := t.TempDir()
	write := func(p, v string) {
		t.Helper()
		path := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(v), 0644); err != nil {
			t.Fatal(err)
		}
	}
	base := "devices/virtual/powercap/intel-rapl/intel-rapl:0"
	write(base+"/name", "package-0")
	write(base+"/energy_uj", "10000000")
	write(base+"/max_energy_range_uj", "100000000")
	write(base+"/constraint_0_name", "long_term")
	write(base+"/constraint_0_power_limit_uw", "45000000")
	write(base+"/constraint_0_time_window_us", "28000000")
	write(base+"/intel-rapl:0:0/name", "core")
	write(base+"/intel-rapl:0:0/energy_uj", "2000000")
	if err := os.MkdirAll(filepath.Join(root, "class/powercap"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, base), filepath.Join(root, "class/powercap/intel-rapl:0")); err != nil {
		t.Fatal(err)
	}
	c := NewCollector()
	r := c.linuxCPUPower(context.Background(), root)
	if len(r.Sensors) != 2 || r.PackageWatts() != nil {
		t.Fatalf("hierarchy/first sample: %+v", r)
	}
	for key, v := range c.energy {
		v.At = time.Now().Add(-2 * time.Second)
		c.energy[key] = v
	}
	write(base+"/energy_uj", "70000000")
	write(base+"/intel-rapl:0:0/energy_uj", "22000000")
	r = c.linuxCPUPower(context.Background(), root)
	watts := r.PackageWatts()
	if watts == nil {
		t.Fatal("package power missing after the second energy sample")
	}
	// Fixture writes and runner scheduling add real time to the two-second
	// baseline. Check the 60-joule delta against the measured interval.
	var interval time.Duration
	for _, sensor := range r.Sensors {
		if sensor.Package {
			interval = sensor.Interval
		}
	}
	if interval < 2*time.Second || math.Abs(*watts*interval.Seconds()-60) > 1e-9 {
		t.Fatalf("wrong package power: %g W over %s", *watts, interval)
	}
	if len(r.Sensors[0].Limits) != 1 || r.Sensors[0].Limits[0].Watts != 45 || r.Sensors[0].Limits[0].Window != 28*time.Second {
		t.Fatal("configured limit units incorrect")
	}
	write(base+"/energy_uj", "unreadable-value")
	r = c.linuxCPUPower(context.Background(), root)
	if r.PackageWatts() != nil || len(r.Problems) == 0 {
		t.Fatal("failed read retained watts")
	}
	write(base+"/energy_uj", "71000000")
	if c.linuxCPUPower(context.Background(), root).PackageWatts() != nil {
		t.Fatal("failed sample baseline retained")
	}
}
func TestLinuxClocksPreferHardwareAndLabelDriverFallback(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "devices/system/cpu/cpufreq/policy0")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"cpuinfo_cur_freq": "2300000", "scaling_cur_freq": "3000000", "scaling_min_freq": "800000", "scaling_max_freq": "4200000", "scaling_governor": "powersave", "affected_cpus": "0 1"} {
		if err := os.WriteFile(filepath.Join(p, k), []byte(v), 0644); err != nil {
			t.Fatal(err)
		}
	}
	r := linuxCPUClocks(context.Background(), root)
	if len(r) != 1 || r[0].MHz != 2300 || r[0].Kind != "Hardware reported" || r[0].MinMHz != 800 || r[0].MaxMHz != 4200 {
		t.Fatalf("bad clocks: %+v", r)
	}
	if err := os.Remove(filepath.Join(p, "cpuinfo_cur_freq")); err != nil {
		t.Fatal(err)
	}
	r = linuxCPUClocks(context.Background(), root)
	if r[0].MHz != 3000 || r[0].Kind != "Driver reported (may be requested)" {
		t.Fatal("driver clock mislabeled actual")
	}
}
