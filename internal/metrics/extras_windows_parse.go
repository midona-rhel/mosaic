package metrics

import (
	"encoding/json"
	"fmt"
)

func parseWindowsBattery(data []byte) BatteryReport {
	var raw struct {
		Devices []struct {
			Name, State                                  string
			Percent, Full, Design, Rate, Voltage, Cycles *float64
			Remaining, Capabilities                      *float64
		}
		Problem string
	}
	if e := json.Unmarshal(data, &raw); e != nil {
		return BatteryReport{Problem: fmt.Sprint(e)}
	}
	r := BatteryReport{Problem: raw.Problem}
	clean := func(v *float64) *float64 {
		if v == nil || *v == 4294967295 || *v == 2147483648 {
			return nil
		}
		return nonnegative(*v)
	}
	for _, v := range raw.Devices {
		b := Battery{Name: v.Name, State: v.State, Source: "Windows battery driver / CIM", Percent: clean(v.Percent), Cycles: clean(v.Cycles)}
		if b.Percent != nil && *b.Percent > 100 {
			b.Percent = nil
		}
		b.Health = ratio(clean(v.Full), clean(v.Design))
		if caps := clean(v.Capabilities); caps != nil && uint64(*caps)&0x40000000 == 0 {
			if now, full := clean(v.Remaining), clean(v.Full); now != nil && full != nil && *full > 0 {
				b.Remaining, b.Full, b.CapacityUnit = nonnegative(*now/1000), nonnegative(*full/1000), "Wh"
			}
		}
		if n := clean(v.Rate); n != nil {
			b.Watts = nonnegative(*n / 1000)
		}
		if n := clean(v.Voltage); n != nil && *n > 0 {
			b.Voltage = nonnegative(*n / 1000)
		}
		r.Devices = append(r.Devices, b)
	}
	return r
}
func parseWindowsFans(data []byte) FanReport {
	var raw struct {
		Devices []struct {
			Name, Source string
			RPM          *float64
		}
		Problem string
	}
	if e := json.Unmarshal(data, &raw); e != nil {
		return FanReport{Problem: e.Error()}
	}
	r := FanReport{Problem: raw.Problem}
	for _, f := range raw.Devices {
		if f.RPM != nil && nonnegative(*f.RPM) != nil && *f.RPM <= 100000 {
			r.Devices = append(r.Devices, Fan{f.Name, f.Source, *f.RPM})
		}
	}
	return r
}
