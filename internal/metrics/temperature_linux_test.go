//go:build linux

package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxCPUTemperatureIdentityUnitsFaultsAndThermalFallback(t *testing.T) {
	root := t.TempDir()
	put := func(path, value string) {
		t.Helper()
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for path, value := range map[string]string{
		"class/hwmon/hwmon0/name": "coretemp", "class/hwmon/hwmon0/temp1_label": "Package id 0", "class/hwmon/hwmon0/temp1_input": "64500", "class/hwmon/hwmon0/temp1_crit": "100000",
		"class/hwmon/hwmon0/temp2_input": "95000", "class/hwmon/hwmon0/temp2_fault": "1",
		"class/hwmon/hwmon1/name": "amdgpu", "class/hwmon/hwmon1/temp1_input": "99000",
		"class/hwmon/hwmon2/device/name": "k10temp", "class/hwmon/hwmon2/device/temp1_label": "Tctl", "class/hwmon/hwmon2/device/temp1_input": "85000", "class/hwmon/hwmon2/device/temp2_label": "Tdie", "class/hwmon/hwmon2/device/temp2_input": "65000",
		"class/thermal/thermal_zone0/type": "acpitz", "class/thermal/thermal_zone0/temp": "95000",
		"class/thermal/thermal_zone1/type": "cpu-thermal", "class/thermal/thermal_zone1/temp": "47000",
	} {
		put(path, value)
	}
	r := linuxCPUThermal(context.Background(), root)
	if len(r.Sensors) != 3 || r.Hottest().Celsius != 65 {
		t.Fatalf("CPU selection/units wrong: %+v", r)
	}
	if r.Sensors[0].Critical == nil || *r.Sensors[0].Critical != 100 {
		t.Fatal("critical threshold lost")
	}
	if err := os.RemoveAll(filepath.Join(root, "class/hwmon")); err != nil {
		t.Fatal(err)
	}
	r = linuxCPUThermal(context.Background(), root)
	if len(r.Sensors) != 1 || r.Hottest().Celsius != 47 {
		t.Fatal("fallback used non-CPU zone or wrong units")
	}
	put("class/thermal/thermal_zone1/temp", "broken")
	if linuxCPUThermal(context.Background(), root).Hottest() != nil {
		t.Fatal("failed reading became zero")
	}
}
