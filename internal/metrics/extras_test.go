package metrics

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"testing"
	"time"
)

func TestAppleBatteryPowerFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		watts        *float64
		source       string
	}{
		{"instant beats stale average", `<key>InstantAmperage</key><integer>-1000</integer><key>Amperage</key><integer>0</integer>`, nonnegative(12), "InstantAmperage"},
		{"unsigned32 discharge", `<key>InstantAmperage</key><integer>4294966296</integer>`, nonnegative(12), "InstantAmperage"},
		{"zero instant tries average", `<key>InstantAmperage</key><integer>0</integer><key>Amperage</key><integer>-500</integer>`, nonnegative(6), "Amperage"},
		{"invalid instant tries average", `<key>InstantAmperage</key><integer>1000000</integer><key>Amperage</key><integer>-500</integer>`, nonnegative(6), "Amperage"},
		{"both zero on battery", `<key>InstantAmperage</key><integer>0</integer><key>Amperage</key><integer>0</integer>`, nil, "Amperage"},
		{"no current", ``, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`<plist><array><dict><key>ExternalConnected</key><false/><key>Voltage</key><integer>12000</integer>%s</dict></array></plist>`, tc.fields)
			b := parseAppleBattery([]byte(payload)).Devices[0]
			if (b.Watts == nil) != (tc.watts == nil) || (b.Watts != nil && *b.Watts != *tc.watts) {
				t.Fatalf("wrong power: %+v", b)
			}
			if tc.source != "" && b.PowerSource != "AppleSmartBattery / "+tc.source+" × Voltage" {
				t.Fatal(b.PowerSource)
			}
			if b.Watts == nil && b.PowerProblem == "" {
				t.Fatal("missing explanation")
			}
		})
	}
	b := parseAppleBattery([]byte(`<plist><array><dict><key>ExternalConnected</key><true/><key>Voltage</key><integer>12000</integer><key>Amperage</key><integer>0</integer><key>AppleRawMaxCapacity</key><integer>5150</integer><key>DesignCapacity</key><integer>5000</integer></dict></array></plist>`)).Devices[0]
	if b.Watts == nil || *b.Watts != 0 || b.PowerProblem != "" || *b.Health != 103 {
		t.Fatalf("valid idle AC power or capacity above design lost: %+v", b)
	}
}

func TestAppleSMCBatteryPowerSelection(t *testing.T) {
	b := Battery{State: "Discharging", Watts: nonnegative(12), PowerSource: "fallback"}
	for _, bad := range []*float64{nil, nonnegative(0), nonnegative(1001), func() *float64 { v := math.NaN(); return &v }()} {
		applyAppleBatterySMCPower(&b, bad)
		if *b.Watts != 12 || b.PowerSource != "fallback" {
			t.Fatal("invalid SMC replaced fallback")
		}
	}
	b.Watts = nil
	b.PowerProblem = "zero current"
	v := -14.5
	applyAppleBatterySMCPower(&b, &v)
	if b.Watts == nil || *b.Watts != 14.5 || b.PowerSource != "AppleSMC / PPBR" || b.PowerProblem != "" {
		t.Fatalf("%+v", b)
	}
	b.State = "On AC power"
	applyAppleBatterySMCPower(&b, nonnegative(0))
	if b.Watts == nil || *b.Watts != 0 {
		t.Fatal("idle AC battery power must allow zero")
	}
}

func TestAppleBatteryUnitsAndSignedCurrent(t *testing.T) {
	r := parseAppleBattery([]byte(`<plist><array><dict>
 <key>BatteryInstalled</key><true/><key>CurrentCapacity</key><integer>78</integer><key>MaxCapacity</key><integer>100</integer>
 <key>AppleRawMaxCapacity</key><integer>4700</integer><key>DesignCapacity</key><integer>5000</integer>
 <key>Voltage</key><integer>12000</integer><key>Amperage</key><integer>18446744073709550616</integer>
 <key>CycleCount</key><integer>143</integer><key>ExternalConnected</key><false/>
 </dict></array></plist>`))
	if r.Problem != "" || len(r.Devices) != 1 {
		t.Fatalf("%+v", r)
	}
	b := r.Devices[0]
	if b.Percent == nil || *b.Percent != 78 || b.Health == nil || *b.Health != 94 || b.Watts == nil || *b.Watts != 12 || b.State != "Discharging" {
		t.Fatalf("wrong units: %+v", b)
	}
	// A percentage MaxCapacity cannot be treated as a capacity for health.
	r = parseAppleBattery([]byte(`<plist><array><dict><key>MaxCapacity</key><integer>100</integer><key>DesignCapacity</key><integer>5000</integer></dict></array></plist>`))
	if r.Devices[0].State != "Unavailable" {
		t.Fatal("missing state was guessed")
	}
	if r.Devices[0].Health != nil || r.Devices[0].Watts != nil || r.Devices[0].Percent != nil {
		t.Fatal("missing fields were invented")
	}
	if r := parseAppleBattery([]byte(`<plist><array/></plist>`)); len(r.Devices) != 0 || r.Problem != "" {
		t.Fatal("desktop is not a provider failure")
	}
	if r := parseAppleBattery([]byte(`broken`)); r.Problem == "" {
		t.Fatal("malformed data accepted")
	}
}
func TestWindowsBatteryUnknownsAndUnits(t *testing.T) {
	r := parseWindowsBattery([]byte(`{"Devices":[{"Name":"BAT0","State":"Discharging","Percent":70,"Full":45000,"Design":50000,"Rate":12500,"Voltage":12000},{"Name":"unknown","Full":4294967295,"Design":50000,"Rate":2147483648,"Voltage":4294967295}]}`))
	a, b := r.Devices[0], r.Devices[1]
	if *a.Health != 90 || *a.Watts != 12.5 || *a.Voltage != 12 || *a.Percent != 70 {
		t.Fatal("incorrect Windows battery units")
	}
	if b.Watts != nil || b.Voltage != nil || b.Health != nil || b.Percent != nil {
		t.Fatal("unknown sentinel displayed as real measurement")
	}
}
func TestFanSMCZeroAndHighRPM(t *testing.T) {
	// 6000 RPM exceeds the power reader's watt limits but is a valid fan speed.
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 6000*4)
	if v := smcNumericValue("fpe2", data); v == nil || *v != 6000 {
		t.Fatal("fan fixed point decoding failed")
	}
	if v := smcNumericValue("fpe2", []byte{0, 0}); v == nil || *v != 0 {
		t.Fatal("stopped fan must remain zero")
	}
	if v := smcPowerValue("fpe2", []byte{0, 0}); v != nil {
		t.Fatal("existing zero-power sentinel behavior changed")
	}
	if v := smcNumericValue("ui8 ", []byte{2}); v == nil || *v != 2 {
		t.Fatal("FNum decoding failed")
	}
}

func TestConnectionsFindOwnListener(t *testing.T) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Skipf("local socket creation unavailable: %v", e)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r := collectConnections(ctx)
	for _, v := range r.Entries {
		if v.Local == l.Addr().String() && v.State == "LISTEN" {
			if v.PID != int32(os.Getpid()) || v.Process == "Unavailable" {
				t.Fatalf("own listener lost ownership: %+v", v)
			}
			return
		}
	}
	t.Fatalf("own listener missing from %d entries: %s", len(r.Entries), r.Problem)
}
func TestWindowsFansDistinguishStoppedAndMissing(t *testing.T) {
	r := parseWindowsFans([]byte(`{"Devices":[{"Name":"stopped","RPM":0},{"Name":"missing"},{"Name":"spinning","RPM":1800},{"Name":"bad","RPM":-1}]}`))
	if len(r.Devices) != 2 || r.Devices[0].RPM != 0 || r.Devices[1].RPM != 1800 {
		t.Fatalf("%+v", r)
	}
}

func TestAppleNestedBatteryCapacities(t *testing.T) {
	r := parseAppleBattery([]byte(`<plist><array><dict><key>BatteryData</key><dict><key>RemainingCapacity</key><integer>2000</integer><key>NominalChargeCapacity</key><integer>4000</integer><key>DesignCapacity</key><integer>5000</integer></dict></dict></array></plist>`))
	if len(r.Devices) != 1 || r.Devices[0].Percent == nil || *r.Devices[0].Percent != 50 || r.Devices[0].Health == nil || *r.Devices[0].Health != 80 {
		t.Fatalf("nested capacity units lost: %+v", r)
	}
	b := r.Devices[0]
	if b.Remaining == nil || *b.Remaining != 2000 || b.Full == nil || *b.Full != 4000 || b.CapacityUnit != "mAh" {
		t.Fatal("raw charge capacities lost or mislabeled as energy")
	}
}

func TestBatteryCapacityUnitsAndMissingValues(t *testing.T) {
	for _, tc := range []struct {
		fields string
		want   bool
	}{
		{`"Capabilities":0,"Remaining":24000,"Full":48000`, true},
		{`"Capabilities":1073741824,"Remaining":24,"Full":48`, false},
		{`"Remaining":24000,"Full":48000`, false},
		{`"Capabilities":0,"Remaining":4294967295,"Full":48000`, false},
		{`"Capabilities":0,"Remaining":0,"Full":48000`, true},
	} {
		b := parseWindowsBattery([]byte(`{"Devices":[{` + tc.fields + `}]}`)).Devices[0]
		if (b.Remaining != nil && b.Full != nil) != tc.want {
			t.Fatalf("wrong Windows capacity availability: %+v", b)
		}
		if tc.want && (*b.Full != 48 || b.CapacityUnit != "Wh" || (*b.Remaining != 24 && *b.Remaining != 0)) {
			t.Fatalf("wrong mWh conversion: %+v", b)
		}
	}
	b := parseAppleBattery([]byte(`<plist><array><dict><key>CurrentCapacity</key><integer>80</integer><key>MaxCapacity</key><integer>100</integer></dict></array></plist>`)).Devices[0]
	if b.Remaining != nil || b.Full != nil {
		t.Fatal("Apple percent was mislabeled as mAh")
	}
	b = parseAppleBattery([]byte(`<plist><array><dict><key>CurrentCapacity</key><integer>2000</integer><key>MaxCapacity</key><integer>4000</integer></dict></array></plist>`)).Devices[0]
	if b.Remaining == nil || *b.Remaining != 2000 || b.Full == nil || *b.Full != 4000 {
		t.Fatal("legacy Apple mAh not retained")
	}
}
