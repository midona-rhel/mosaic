package metrics

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Adapted from macmon's MIT-licensed CLPC catalog and energy conversion.
// Report IDs extracted for macOS 27; unknown drivers/versions are not guessed.
// See THIRD_PARTY_NOTICES and docs/telemetry.md for pinned source references.
func appleLegacyEnergyBlocked(version string) bool {
	major, err := strconv.Atoi(strings.Split(version, ".")[0])
	return err != nil || major >= 27
}

func appleCLPCCPUReportID(bundle, version string) (uint64, bool) {
	if strings.Split(version, ".")[0] != "27" {
		return 0, false
	}
	keys := map[string]uint32{
		"com.apple.driver.AppleT6000CLPCv3": 0x47479059,
		"com.apple.driver.AppleT6002CLPC":   0xb501816b,
		"com.apple.driver.AppleT6020CLPC":   0xe301cd75,
		"com.apple.driver.AppleT6022CLPC":   0x76f5fd21,
		"com.apple.driver.AppleT6030CLPC":   0xbe03a01a,
		"com.apple.driver.AppleT6031CLPC":   0xd81e0084,
		"com.apple.driver.AppleT6032CLPC":   0xf4a21308,
		"com.apple.driver.AppleT6041CLPC":   0x263cf1f0,
		"com.apple.driver.AppleT6050CLPC":   0xd3ab60bb,
		"com.apple.driver.AppleT6050dCLPC":  0x18b3010e,
		"com.apple.driver.AppleT8103CLPCv3": 0x2cdd31f2,
		"com.apple.driver.AppleT8112CLPC":   0xb5435137,
		"com.apple.driver.AppleT8122CLPC":   0x54b51c99,
		"com.apple.driver.AppleT8132CLPC":   0x4a854d94,
		"com.apple.driver.AppleT8140CLPC":   0x9bf54436,
		"com.apple.driver.AppleT8142CLPC":   0x43708744,
		"com.apple.driver.AppleT8152CLPC":   0x485d4bfc,
	}
	key, ok := keys[bundle]
	if !ok {
		return 0, false
	}
	return uint64(16)<<32 | uint64(key), true
}

type appleEnergyChannel struct {
	Driver, ID                  uint64
	Group, Subgroup, Name, Unit string
	Energy                      int64
}

func (c appleEnergyChannel) identity() string { return fmt.Sprintf("%x:%x", c.Driver, c.ID) }
func (c appleEnergyChannel) cpuSource() string {
	if c.Group == "CLPC" && c.Subgroup == "Energy Counters" && c.Name == "CPU Energy" && c.Unit == "nJ" {
		return "CLPC"
	}
	if c.Group != "Energy Model" || (c.Unit != "mJ" && c.Unit != "uJ" && c.Unit != "nJ") {
		return ""
	}
	if c.Name == "CPU Energy" {
		return "Energy Model"
	}
	if strings.HasPrefix(c.Name, "DIE_") && strings.HasSuffix(c.Name, "_CPU Energy") {
		middle := strings.TrimSuffix(strings.TrimPrefix(c.Name, "DIE_"), "_CPU Energy")
		if middle == "" {
			return ""
		}
		for _, r := range middle {
			if r < '0' || r > '9' {
				return ""
			}
		}
		return "Energy Model"
	}
	return ""
}
func appleEnergyWatts(energy int64, unit string, dt time.Duration) *float64 {
	if energy < 0 || dt <= 0 {
		return nil
	}
	scale := map[string]float64{"mJ": 1e3, "uJ": 1e6, "nJ": 1e9}[unit]
	if scale == 0 {
		return nil
	}
	watts := float64(energy) / scale / dt.Seconds()
	if !validPower(watts) {
		return nil
	}
	return &watts
}

// Expected channels come from the subscription, never the position/order of a
// sample. A missing/invalid component leaves the package total unavailable.
func appleEnergyReport(expected, delta []appleEnergyChannel, version string, dt time.Duration) CPUPowerReport {
	r := CPUPowerReport{At: time.Now()}
	source := "Energy Model"
	for _, c := range expected {
		if c.cpuSource() == "CLPC" {
			source = "CLPC"
			break
		}
	}
	if appleLegacyEnergyBlocked(version) && source != "CLPC" {
		r.Problems = []string{"No supported AppleCLPC CPU energy channel. Legacy Energy Model counters can freeze on macOS 27 and later; they are deliberately not shown as 0 W."}
		return r
	}
	// Prefer a total over per-die components if both are advertised.
	aggregate := false
	for _, c := range expected {
		if c.cpuSource() == source && c.Name == "CPU Energy" {
			aggregate = true
		}
	}
	values := map[string]appleEnergyChannel{}
	for _, c := range delta {
		values[c.identity()] = c
	}
	seen := map[string]bool{}
	for _, c := range expected {
		if c.cpuSource() != source || (aggregate && c.Name != "CPU Energy") {
			continue
		}
		identity := c.identity()
		alias := identity
		if source == "CLPC" {
			alias = fmt.Sprintf("%x:%x", c.Driver, uint32(c.ID))
		}
		if seen[alias] {
			continue
		}
		seen[alias] = true
		s := CPUPower{ID: identity, Name: c.Name, Package: true, Source: "Apple IOReport / " + source + " / macOS " + version + " / " + identity, Interval: dt, State: "Unavailable"}
		if v, ok := values[identity]; ok && v.cpuSource() == source && v.Unit == c.Unit && v.Name == c.Name {
			s.Watts = appleEnergyWatts(v.Energy, v.Unit, dt)
		}
		if s.Watts != nil {
			s.State = ""
		} else {
			r.Problems = append(r.Problems, "Missing or invalid CPU energy delta: "+identity)
		}
		r.Sensors = append(r.Sensors, s)
	}
	if len(r.Sensors) == 0 {
		r.Problems = append(r.Problems, "IOReport returned no recognized CPU energy channel.")
	}
	return r
}
