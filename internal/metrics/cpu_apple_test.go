package metrics

import (
	"slices"
	"testing"
	"time"
)

func TestM5TemperatureKeysAreModelMatched(t *testing.T) {
	for _, model := range []string{"Apple M5", "Apple M5 Pro", "Apple M5 Max"} {
		keys := appleCPUTemperatureKeys(model)
		if !slices.Contains(keys, "Tp00") || !slices.Contains(keys, "Tp0y") || len(keys) != 20 {
			t.Fatalf("M5 key set missing: %s %v", model, keys)
		}
		if slices.Contains(keys, "Tp01") || slices.Contains(keys, "Tg00") {
			t.Fatal("another generation or GPU sensor added")
		}
	}
	for _, model := range []string{"Intel Core i9", "Apple M50", "Unknown"} {
		if slices.Contains(appleCPUTemperatureKeys(model), "Tp0P") || slices.Contains(appleCPUTemperatureKeys(model), "Tp00") {
			t.Fatal("CPU key mapping leaked across models")
		}
	}
	if !slices.Contains(appleCPUTemperatureKeys("Apple M1 Pro"), "Tp0P") {
		t.Fatal("valid M1 CPU sensor excluded")
	}
}

func TestCLPCIDsAreSpecificToDriverAndOS(t *testing.T) {
	for _, tc := range []struct {
		bundle string
		id     uint64
	}{
		{"com.apple.driver.AppleT6002CLPC", 0x00000010b501816b},
		{"com.apple.driver.AppleT8112CLPC", 0x00000010b5435137},
		{"com.apple.driver.AppleT8142CLPC", 0x0000001043708744},
		{"com.apple.driver.AppleT6050dCLPC", 0x0000001018b3010e},
	} {
		id, ok := appleCLPCCPUReportID(tc.bundle, "27.0.1")
		if !ok || id != tc.id {
			t.Fatalf("bad CLPC mapping: %s %x", tc.bundle, id)
		}
		for _, version := range []string{"", "26.6", "28.0", "127.0"} {
			if _, ok := appleCLPCCPUReportID(tc.bundle, version); ok {
				t.Fatal("guessed counter ID on another OS")
			}
		}
	}
	if _, ok := appleCLPCCPUReportID("com.apple.driver.UnknownCLPC", "27.0"); ok {
		t.Fatal("unknown driver guessed")
	}
}

func TestLegacyEnergyIsBlockedOnNewOrUnknownVersions(t *testing.T) {
	for _, version := range []string{"27.0", "28.0", ""} {
		if !appleLegacyEnergyBlocked(version) {
			t.Fatal("unsafe legacy fallback on", version)
		}
	}
	if appleLegacyEnergyBlocked("26.6.2") {
		t.Fatal("legacy supported version blocked")
	}
}

func TestAppleEnergyUnitsAndInvalidDeltas(t *testing.T) {
	for _, tc := range []struct {
		energy int64
		unit   string
	}{{6250, "mJ"}, {6250000, "uJ"}, {6250000000, "nJ"}} {
		w := appleEnergyWatts(tc.energy, tc.unit, 250*time.Millisecond)
		if w == nil || *w != 25 {
			t.Fatalf("bad conversion: %+v %v", tc, w)
		}
	}
	for _, tc := range []struct {
		energy int64
		unit   string
		dt     time.Duration
	}{{-1, "nJ", time.Second}, {1, "events", time.Second}, {1, "nJ", 0}} {
		if appleEnergyWatts(tc.energy, tc.unit, tc.dt) != nil {
			t.Fatal("invalid delta accepted")
		}
	}
	if w := appleEnergyWatts(0, "nJ", time.Second); w == nil || *w != 0 {
		t.Fatal("valid idle CLPC zero lost")
	}
}

func TestAppleEnergySelectionNeverUsesFrozenLegacyOrPartialTotal(t *testing.T) {
	clpc := appleEnergyChannel{Driver: 1, ID: 100, Group: "CLPC", Subgroup: "Energy Counters", Name: "CPU Energy", Unit: "nJ", Energy: 2500000000}
	legacy := appleEnergyChannel{Driver: 2, ID: 200, Group: "Energy Model", Name: "CPU Energy", Unit: "mJ", Energy: 9000}
	gpu := clpc
	gpu.ID = 101
	gpu.Name = "GPU Energy"
	gpu.Energy = 9999999999
	r := appleEnergyReport([]appleEnergyChannel{legacy, clpc, gpu}, []appleEnergyChannel{gpu, legacy, clpc}, "27.0", 250*time.Millisecond)
	if w := r.PackageWatts(); w == nil || *w != 10 || len(r.Sensors) != 1 {
		t.Fatalf("mixed/positional power: %+v", r)
	}
	r = appleEnergyReport([]appleEnergyChannel{legacy}, []appleEnergyChannel{legacy}, "27.0", time.Second)
	if r.PackageWatts() != nil || len(r.Problems) == 0 {
		t.Fatal("macOS 27 accepted legacy counters")
	}
	bad := clpc
	bad.Energy = -1
	r = appleEnergyReport([]appleEnergyChannel{clpc, legacy}, []appleEnergyChannel{bad, legacy}, "27.0", time.Second)
	if r.PackageWatts() != nil {
		t.Fatal("invalid CLPC silently fell back to legacy")
	}
	die0, die1 := legacy, legacy
	die0.Name = "DIE_0_CPU Energy"
	die1.Name = "DIE_1_CPU Energy"
	die1.ID = 201
	r = appleEnergyReport([]appleEnergyChannel{die0, die1}, []appleEnergyChannel{die0}, "26.0", time.Second)
	if r.PackageWatts() != nil {
		t.Fatal("missing die displayed as total")
	}
	r = appleEnergyReport([]appleEnergyChannel{legacy, die0, die1}, []appleEnergyChannel{legacy, die1}, "26.0", time.Second)
	if w := r.PackageWatts(); w == nil || *w != 9 {
		t.Fatal("aggregate and die counters double counted")
	}
	alias := clpc
	alias.ID = uint64(17)<<32 | clpc.ID
	r = appleEnergyReport([]appleEnergyChannel{clpc, alias}, []appleEnergyChannel{alias, clpc}, "27.0", time.Second)
	if w := r.PackageWatts(); w == nil || *w != 2.5 {
		t.Fatal("CLPC alias counted twice")
	}
}
