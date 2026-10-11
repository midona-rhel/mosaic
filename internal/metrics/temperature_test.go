package metrics

import (
	"github.com/shirou/gopsutil/v4/sensors"
	"math"
	"testing"
)

func TestCPUThermalSummaryPreservesControlSemantics(t *testing.T) {
	r := CPUThermalReport{Sensors: []CPUTemperature{{Name: "Tctl", Celsius: 95, Control: true}, {Name: "Tdie", Celsius: 65}, {Name: "Core", Celsius: 70}, {Name: "Broken", Celsius: math.NaN()}}}
	if got := r.Hottest(); got == nil || got.Name != "Core" {
		t.Fatalf("control offset or invalid value became headline: %+v", got)
	}
	r.Sensors = r.Sensors[:1]
	if got := r.Hottest(); got == nil || !got.Control {
		t.Fatal("control-only fallback was lost")
	}
	if (CPUThermalReport{}).Hottest() != nil {
		t.Fatal("unavailable became zero")
	}
}
func TestAppleCPUSensorsExcludeGPUProximityAndMissingZero(t *testing.T) {
	values := []sensors.TemperatureStat{{SensorKey: "TC0D", Temperature: 64.5}, {SensorKey: "TC0P", Temperature: 45}, {SensorKey: "TG0D", Temperature: 98}, {SensorKey: "pACC MTR Temp Sensor0", Temperature: 62}, {SensorKey: "eACC MTR Temp Sensor0", Temperature: 48}, {SensorKey: "GPU MTR Temp Sensor0", Temperature: 92}, {SensorKey: "CPU missing", Temperature: 0}}
	intel := appleCPUThermal(values, "amd64")
	arm := appleCPUThermal(values, "arm64")
	if len(intel.Sensors) != 1 || intel.Hottest().Celsius != 64.5 {
		t.Fatal("Intel CPU sensor selection incorrect")
	}
	if len(arm.Sensors) != 2 || arm.Hottest().Celsius != 62 {
		t.Fatal("Apple Silicon CPU sensor selection incorrect")
	}
}
func TestWindowsCPUThermalNullAndProviderZeroAreUnavailable(t *testing.T) {
	r, err := parseWindowsCPUThermal([]byte(`{"Sensors":[{"ID":"/cpu/0/temp/0","Name":"CPU Package","Hardware":"CPU model","Source":"root/LibreHardwareMonitor","Value":68.25},{"ID":"zero","Value":0},{"ID":"missing","Value":null},{"ID":"invalid","Value":999}],"Problems":[]}`))
	if err != nil || len(r.Sensors) != 1 || r.Hottest().Celsius != 68.25 {
		t.Fatalf("bad sensor decode: %+v %v", r, err)
	}
	if _, err := parseWindowsCPUThermal([]byte("broken")); err == nil {
		t.Fatal("malformed provider data accepted")
	}
}
