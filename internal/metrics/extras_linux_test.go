//go:build linux

package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func fixtureFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, v := range files {
		p = filepath.Join(root, p)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(v), 0644); e != nil {
			t.Fatal(e)
		}
	}
}
func TestLinuxBatteryCapacityFamilies(t *testing.T) {
	root := t.TempDir()
	fixtureFiles(t, root, map[string]string{
		"BAT0/type": "Battery", "BAT0/status": "Discharging", "BAT0/energy_now": "24000000", "BAT0/energy_full": "48000000", "BAT0/energy_full_design": "60000000", "BAT0/power_now": "12500000",
		"BAT1/type": "Battery", "BAT1/charge_now": "1500000", "BAT1/charge_full": "3000000", "BAT1/charge_full_design": "4000000", "BAT1/current_now": "-1000000", "BAT1/voltage_now": "12000000",
		"mouse/type": "Battery", "mouse/scope": "Device", "mouse/capacity": "99",
	})
	r := linuxBattery(context.Background(), root)
	if len(r.Devices) != 2 {
		t.Fatalf("peripheral battery was included: %+v", r)
	}
	a, b := r.Devices[0], r.Devices[1]
	if a.Remaining == nil || *a.Remaining != 24 || a.Full == nil || *a.Full != 48 || a.CapacityUnit != "Wh" {
		t.Fatal("remaining/full energy units wrong")
	}
	if b.Remaining == nil || *b.Remaining != 1500 || b.Full == nil || *b.Full != 3000 || b.CapacityUnit != "mAh" {
		t.Fatal("remaining/full charge units wrong")
	}
	if *a.Percent != 50 || *a.Health != 80 || *a.Watts != 12.5 {
		t.Fatal("microWh/microW conversion wrong")
	}
	if *b.Percent != 50 || *b.Health != 75 || *b.Watts != 12 {
		t.Fatal("microAh and microA conversion wrong")
	}
	if a.Voltage != nil || a.Cycles != nil {
		t.Fatal("missing fields were synthesized")
	}
}
func TestLinuxFansAcceptStoppedButRejectInvalid(t *testing.T) {
	root := t.TempDir()
	fixtureFiles(t, root, map[string]string{"hwmon0/name": "chip", "hwmon0/fan1_input": "0", "hwmon0/fan1_label": "CPU fan", "hwmon0/fan2_input": "1850", "hwmon0/fan3_input": "-1", "hwmon0/fan4_input": "bad"})
	r := linuxFans(context.Background(), root)
	if len(r.Devices) != 2 || r.Devices[0].RPM != 0 || r.Devices[1].RPM != 1850 || r.Devices[0].Name != "chip / CPU fan" {
		t.Fatalf("bad fan data: %+v", r)
	}
}
