//go:build linux

package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDRMDeviceDiscoveryAndAMDSensors(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "devices", "0000:01:00.0")
	if err := os.MkdirAll(filepath.Join(device, "hwmon", "hwmon1"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"vendor": "0x1002", "device": "0x744c", "gpu_busy_percent": "75", "mem_info_vram_total": "8589934592", "mem_info_vram_used": "1073741824", "hwmon/hwmon1/temp1_input": "54000", "hwmon/hwmon1/power1_average": "18500000"}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(device, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"card0", "card1", "card0-DP-1"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(device, filepath.Join(root, name, "device")); err != nil {
			t.Fatal(err)
		}
	}
	r, err := linuxDRM(root)
	if err != nil || len(r) != 1 {
		t.Fatalf("duplicate/connector detection: %v %+v", err, r)
	}
	g := r[0]
	if *g.Utilization != 75 || *g.MemoryUsed != 1<<30 || *g.MemoryTotal != 8<<30 || *g.Temperature != 54 || *g.Power != 18.5 {
		t.Fatalf("wrong sysfs units: %+v", g)
	}
	if err := os.WriteFile(filepath.Join(device, "mem_info_vram_total"), []byte("0"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err = linuxDRM(root)
	if err != nil || r[0].MemoryTotal != nil || r[0].MemoryUsed != nil {
		t.Fatal("zero dedicated capacity reported as usable VRAM")
	}
}
