package metrics

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
)

func plistEntries(data []byte) ([]any, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if s, ok := t.(xml.StartElement); ok && s.Name.Local == "array" {
			v, e := plistValue(d, s, 0)
			if e != nil {
				return nil, e
			}
			if a, ok := v.([]any); ok {
				return a, nil
			}
		}
	}
	return nil, fmt.Errorf("missing IORegistry plist array")
}
func appleNumber(m map[string]any, k string) *float64 {
	v, ok := m[k]
	if !ok {
		return nil
	}
	return finiteNumber(fmt.Sprint(v))
}
func parseAppleBattery(data []byte) BatteryReport {
	r := BatteryReport{}
	entries, e := plistEntries(data)
	if e != nil {
		r.Problem = e.Error()
		return r
	}
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if present, ok := m["BatteryInstalled"].(bool); ok && !present {
			continue
		}
		charging, chargingKnown := m["IsCharging"].(bool)
		online, onlineKnown := m["ExternalConnected"].(bool)
		state := "Unavailable"
		if onlineKnown {
			state = batteryState(charging, !online, online)
		} else if chargingKnown && charging {
			state = "Charging"
		}
		b := Battery{Name: "Internal battery", Source: "IOKit / AppleSmartBattery", State: state}
		b.Percent = ratio(appleNumber(m, "CurrentCapacity"), appleNumber(m, "MaxCapacity"))
		if b.Percent != nil && *b.Percent > 100 {
			b.Percent = nil
		}
		full := appleNumber(m, "AppleRawMaxCapacity")
		if full == nil {
			full = appleNumber(m, "NominalChargeCapacity")
		}
		// On Apple Silicon MaxCapacity may be a percent, not mAh. Only use it
		// as a capacity fallback when its units are unambiguous (>100 mAh).
		if full == nil {
			if v := appleNumber(m, "MaxCapacity"); v != nil && *v > 100 {
				full = v
			}
		}
		data, _ := m["BatteryData"].(map[string]any)
		if full == nil {
			full = appleNumber(data, "NominalChargeCapacity")
		}
		if full == nil {
			full = appleNumber(data, "FullChargeCapacity")
		}
		design := appleNumber(m, "DesignCapacity")
		if design == nil {
			design = appleNumber(data, "DesignCapacity")
		}
		b.Health = ratio(full, design)
		current := appleNumber(m, "AppleRawCurrentCapacity")
		if current == nil {
			current = appleNumber(data, "RemainingCapacity")
		}
		if current == nil {
			// Legacy Intel batteries expose mAh directly, rather than percent.
			if v := appleNumber(m, "MaxCapacity"); v != nil && *v > 100 {
				current = appleNumber(m, "CurrentCapacity")
			}
		}
		b.Remaining, b.Full, b.CapacityUnit = current, full, "mAh"
		if b.Percent == nil {
			b.Percent = ratio(current, full)
			if b.Percent != nil && *b.Percent > 100 {
				b.Percent = nil
			}
		}
		b.Cycles = appleNumber(m, "CycleCount")
		if v := appleNumber(m, "Voltage"); v != nil && *v > 0 {
			b.Voltage = nonnegative(*v / 1000)
		}
		// Prefer the instantaneous current; Amperage is an averaged reading.
		// A zero during discharge may be a stale/unpopulated counter: try the
		// other source, then explicitly report the inconsistency.
		for _, key := range []string{"InstantAmperage", "Amperage"} {
			amps := appleBatteryCurrent(m[key])
			if amps == nil || b.Voltage == nil {
				continue
			}
			b.Watts = nonnegative(math.Abs(*amps) / 1000 * *b.Voltage)
			b.PowerSource = "AppleSmartBattery / " + key + " × Voltage"
			if *b.Watts != 0 || b.State != "Discharging" {
				break
			}
		}
		validateAppleBatteryPower(&b)
		r.Devices = append(r.Devices, b)
	}
	return r
}

func appleBatteryCurrent(value any) *float64 {
	var n int64
	switch v := value.(type) {
	case int64:
		n = v
	case uint64:
		// IORegistry can serialize signed current using a 32- or 64-bit
		// unsigned two's-complement representation.
		if v <= math.MaxUint32 {
			n = int64(int32(uint32(v)))
		} else {
			n = int64(v)
		}
	default:
		return nil
	}
	if n <= -100000 || n >= 100000 {
		return nil
	}
	v := float64(n)
	return &v
}

func validateAppleBatteryPower(b *Battery) {
	b.PowerProblem = ""
	if b.Watts != nil && *b.Watts == 0 && b.State == "Discharging" {
		b.Watts = nil
		b.PowerProblem = "Battery reports discharging but the power reading is zero; waiting for a usable reading."
	} else if b.Watts == nil {
		b.PowerProblem = "No usable battery power or current/voltage reading."
	}
}

// PPBR measures signed battery power on Apple Silicon (also used by Stats).
// Keep a usable current × voltage fallback if SMC is absent, invalid or zero
// while discharging. Never substitute adapter power or CPU package power.
func applyAppleBatterySMCPower(b *Battery, watts *float64) {
	if watts == nil || math.IsNaN(*watts) || math.IsInf(*watts, 0) || math.Abs(*watts) > 1000 {
		return
	}
	if *watts == 0 && b.State == "Discharging" {
		return
	}
	b.Watts = nonnegative(math.Abs(*watts))
	b.PowerSource = "AppleSMC / PPBR"
	b.PowerProblem = ""
}
