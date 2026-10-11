package metrics

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNVIDIAOptionalFieldsAndUnits(t *testing.T) {
	// Representative CSV schema from nvidia-smi's documented selective query.
	r, err := parseNVIDIA("GPU-1, Example GPU, 00000000:01:00.0, 45, 1024, 8192, N/A, [Not Supported]\nGPU-2, Other GPU, 00000000:02:00.0, N/A, N/A, N/A, 52, 97.5\n")
	if err != nil || len(r) != 2 {
		t.Fatalf("parse: %v %+v", err, r)
	}
	if *r[0].MemoryUsed != 1<<30 || *r[0].MemoryTotal != 8<<30 || *r[0].Utilization != 45 {
		t.Fatal("incorrect CSV units")
	}
	if r[0].Temperature != nil || r[0].Power != nil || r[1].Utilization != nil || r[1].MemoryTotal != nil {
		t.Fatal("unavailable fields became zero")
	}
	if r[0].PCI != "0000:01:00.0" {
		t.Fatal("PCI domain normalization failed")
	}
	for _, invalid := range []string{"NaN", "+Inf", "-1", "N/A", "101"} {
		if percentNumber(invalid) != nil {
			t.Fatalf("invalid percentage %q accepted", invalid)
		}
	}
	if _, err := parseNVIDIA("broken,row\n"); err == nil {
		t.Fatal("malformed provider output accepted")
	}
}

func TestWDDMBusiestEngineNotAverage(t *testing.T) {
	// Schema fixture, not a captured Windows measurement. Two process instances
	// share engine 0; engine 1 is separate. Result must be max(25+35,10) = 60.
	r, err := parseWindowsGPU([]byte(`{"Inventory":[{"Name":"Example GPU","PNPDeviceID":"PCI\\example"}],"Engines":[{"Name":"pid_1_luid_0x0_0xAB_phys_0_eng_0_engtype_3D","UtilizationPercentage":25},{"Name":"pid_2_luid_0x0_0xAB_phys_0_eng_0_engtype_3D","UtilizationPercentage":35},{"Name":"pid_3_luid_0x0_0xAB_phys_0_eng_1_engtype_Copy","UtilizationPercentage":10},{"Name":"bad","UtilizationPercentage":90}],"Memory":[{"Name":"luid_0x0_0xAB_phys_0","DedicatedUsage":8589934592,"SharedUsage":1024}],"Problems":[]}`))
	if err != nil || len(r.Devices) != 2 {
		t.Fatalf("parse: %v %+v", err, r)
	}
	g := r.Devices[1]
	if g.Utilization == nil || *g.Utilization != 60 {
		t.Fatalf("engine aggregation: %+v", g)
	}
	if g.MemoryUsed == nil || *g.MemoryUsed != 8<<30 || g.MemoryTotal != nil {
		t.Fatal("64-bit adapter allocation or unknown capacity incorrect")
	}
	if r.Devices[0].Utilization != nil {
		t.Fatal("invented inventory-to-LUID association")
	}
	if _, err := parseWindowsGPU([]byte("garbage")); err == nil {
		t.Fatal("invalid CIM JSON accepted")
	}
}

func TestAppleOptionalDriverProperties(t *testing.T) {
	data := `<?xml version="1.0"?><plist version="1.0"><array><dict><key>IORegistryEntryName</key><string>AGX</string><key>IORegistryEntryID</key><integer>123</integer><key>PerformanceStatistics</key><dict><key>Device Utilization %</key><integer>63</integer><key>Temperature(C)</key><integer>51</integer><key>In use system memory</key><integer>1024</integer></dict></dict><dict><key>IORegistryEntryName</key><string>Unreported accelerator</string></dict></array></plist>`
	r, err := parseAppleGPU([]byte(data))
	if err != nil || len(r) != 2 {
		t.Fatalf("parse: %v %+v", err, r)
	}
	if r[0].Utilization == nil || *r[0].Utilization != 63 || r[0].MemoryTotal != nil || r[0].MemoryUsed != nil {
		t.Fatal("unified memory incorrectly claimed as VRAM")
	}
	if r[1].Utilization != nil {
		t.Fatal("missing driver field converted to zero")
	}
	if _, err := parseAppleGPU([]byte(`<plist><array><dict>`)); err == nil {
		t.Fatal("truncated plist accepted")
	}
}

func TestGPUCacheCancellationAndOutputBound(t *testing.T) {
	c := NewCollector()
	c.gpu = GPUReport{At: time.Now(), Devices: []GPU{{ID: "cached", Name: "Cached fixture"}}}
	if r := c.sampleGPU(context.Background()); len(r.Devices) != 1 || r.Devices[0].ID != "cached" {
		t.Fatal("fresh GPU sample was not cached")
	}
	c.gpu.At = time.Now().Add(-time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := c.sampleGPU(ctx); len(r.Problems) == 0 {
		t.Fatal("cancelled GPU query returned valid data")
	}
	var out limitedOutput
	if _, err := out.Write([]byte(strings.Repeat("x", (2<<20)+1))); err == nil {
		t.Fatal("provider output limit not enforced")
	}
}
