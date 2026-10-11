package metrics

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
	"unsafe"
)

func TestPowerEnergyDeltaAndRebaseline(t *testing.T) {
	at := time.Now()
	previous := energyCounter{Energy: 10e6, Range: 100e6, At: at}
	current := energyCounter{Energy: 70e6, Range: 100e6, At: at.Add(2 * time.Second)}
	w, dt := energyWatts(current, previous)
	if w == nil || *w != 30 || dt != 2*time.Second {
		t.Fatalf("microjoules/time: %v %v", w, dt)
	}
	for _, bad := range []energyCounter{
		{Energy: 1e6, Range: 100e6, At: current.At},   // wrap or reset
		{Energy: 70e6, Range: 200e6, At: current.At},  // changed device range
		{Energy: 70e6, Range: 100e6, At: at},          // zero interval
		{Energy: 101e6, Range: 100e6, At: current.At}, // invalid counter
	} {
		if w, _ := energyWatts(bad, previous); w != nil {
			t.Fatal("invalid transition produced watts")
		}
	}
	if w, _ := energyWatts(current, energyCounter{}); w != nil {
		t.Fatal("first sample produced watts")
	}
	current.Energy = previous.Energy
	if w, _ := energyWatts(current, previous); w == nil || *w != 0 {
		t.Fatal("unchanged valid counter should report measured zero")
	}
}
func TestPowerSummaryNeverAddsOverlappingDomainsOrPartialPackages(t *testing.T) {
	a, b, core := 30., 20., 15.
	r := CPUPowerReport{Sensors: []CPUPower{{ID: "pkg0", Package: true, Watts: &a}, {ID: "pkg1", Package: true, Watts: &b}, {ID: "core0", Watts: &core}, {ID: "pkg0", Package: true, Watts: &a}}}
	if w := r.PackageWatts(); w == nil || *w != 50 {
		t.Fatal("overlap or duplicate counted")
	}
	r.Sensors[1].Watts = nil
	if r.PackageWatts() != nil {
		t.Fatal("partial socket power labeled total")
	}
	if (CPUPowerReport{Sensors: []CPUPower{{ID: "core", Watts: &core}}}).PackageWatts() != nil {
		t.Fatal("core domain mislabeled package")
	}
}
func TestWindowsCPUHardwareTypesAndMissingValues(t *testing.T) {
	r, err := parseWindowsCPUHardware([]byte(`{"Sensors":[
	{"ID":"p0","Type":"Power","Name":"CPU Package","Parent":"cpu0","Value":35.5},
	{"ID":"core","Type":"Power","Name":"CPU Cores","Parent":"cpu0","Value":20},
	{"ID":"clock","Type":"Clock","Name":"CPU Core #1","Value":3400},
	{"ID":"bus","Type":"Clock","Name":"Bus Speed","Value":100},
	{"ID":"temp","Type":"Temperature","Name":"CPU Package","Value":65}]}`))
	if err != nil || len(r.Power.Sensors) != 2 || len(r.Clocks) != 1 || r.Power.PackageWatts() == nil || *r.Power.PackageWatts() != 35.5 {
		t.Fatalf("bad typed report: %+v %v", r, err)
	}
	for _, v := range []string{"null", "0", "-1"} {
		r, err := parseWindowsCPUHardware([]byte(`{"Sensors":[{"ID":"p0","Type":"Power","Name":"Package","Parent":"cpu0","Value":20},{"ID":"p1","Type":"Power","Name":"Package","Parent":"cpu1","Value":` + v + `}]}`))
		if err != nil || len(r.Power.Sensors) != 2 || r.Power.PackageWatts() != nil {
			t.Fatal("missing socket became partial total")
		}
	}
	temp, err := parseWindowsCPUThermal([]byte(`{"Sensors":[{"ID":"p","Type":"Power","Value":35},{"ID":"t","Type":"Temperature","Value":65}]}`))
	if err != nil || len(temp.Sensors) != 1 || temp.Sensors[0].Celsius != 65 {
		t.Fatal("watts misread as degrees")
	}
}
func TestSMCPowerFormatsAndABI(t *testing.T) {
	var s smcParam
	if unsafe.Sizeof(s) != 80 || unsafe.Offsetof(s.Bytes) != 48 || unsafe.Offsetof(s.Operation) != 42 {
		t.Fatal("AppleSMC ABI layout changed")
	}
	var f [4]byte
	binary.LittleEndian.PutUint32(f[:], math.Float32bits(24.5))
	for _, tc := range []struct {
		kind string
		data []byte
		want float64
	}{
		{"flt ", f[:], 24.5}, {"fp88", []byte{24, 128}, 24.5}, {"sp78", []byte{24, 128}, 24.5}, {"fpe2", []byte{0, 98}, 24.5},
	} {
		if v := smcPowerValue(tc.kind, tc.data); v == nil || *v != tc.want {
			t.Fatalf("%s decoding failed", tc.kind)
		}
	}
	for _, tc := range []struct {
		kind string
		data []byte
	}{
		{"flt ", []byte{0, 0, 192, 127}}, {"flt ", []byte{0, 0, 0, 0}}, {"sp78", []byte{255, 0}}, {"fp00", []byte{1, 2}}, {"wat?", f[:]}, {"flt ", []byte{1}},
	} {
		if smcPowerValue(tc.kind, tc.data) != nil {
			t.Fatal("unsupported/missing/invalid SMC value accepted")
		}
	}
}
