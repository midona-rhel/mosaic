//go:build darwin

package metrics

import (
	"context"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

var smcAPI struct {
	once     sync.Once
	err      error
	matching func(string) uintptr
	services func(uint32, uintptr, *uint32) int32
	next     func(uint32) uint32
	name     func(uint32, unsafe.Pointer) int32
	task     func() uint32
	open     func(uint32, uint32, uint32, *uint32) int32
	close    func(uint32) int32
	release  func(uint32) int32
	call     func(uint32, uint32, unsafe.Pointer, uintptr, unsafe.Pointer, *uintptr) int32
}

func initSMCAPI() {
	lib, err := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		smcAPI.err = err
		return
	}
	for _, e := range []struct {
		name string
		fn   any
	}{
		{"IOServiceMatching", &smcAPI.matching}, {"IOServiceGetMatchingServices", &smcAPI.services},
		{"IOIteratorNext", &smcAPI.next}, {"IORegistryEntryGetName", &smcAPI.name},
		{"mach_task_self", &smcAPI.task}, {"IOServiceOpen", &smcAPI.open}, {"IOServiceClose", &smcAPI.close},
		{"IOObjectRelease", &smcAPI.release}, {"IOConnectCallStructMethod", &smcAPI.call},
	} {
		p, err := purego.Dlsym(lib, e.name)
		if err != nil {
			smcAPI.err = err
			return
		}
		purego.RegisterFunc(e.fn, p)
	}
}

type smcConnection struct {
	port     uint32
	endpoint string
}

func openSMC() (*smcConnection, error) {
	smcAPI.once.Do(initSMCAPI)
	if smcAPI.err != nil {
		return nil, smcAPI.err
	}
	var iterator uint32
	if code := smcAPI.services(0, smcAPI.matching("AppleSMC"), &iterator); code != 0 {
		return nil, fmt.Errorf("AppleSMC enumeration: 0x%x", uint32(code))
	}
	if iterator == 0 {
		return nil, fmt.Errorf("AppleSMC service unavailable")
	}
	defer smcAPI.release(iterator)
	type candidate struct {
		entry uint32
		name  string
	}
	var candidates []candidate
	defer func() {
		for _, c := range candidates {
			smcAPI.release(c.entry)
		}
	}()
	for len(candidates) < 64 {
		entry := smcAPI.next(iterator)
		if entry == 0 {
			break
		}
		var name [128]byte
		smcAPI.name(entry, unsafe.Pointer(&name[0]))
		candidates = append(candidates, candidate{entry, cString(name[:])})
	}
	// Apple Silicon has several AppleSMC services; its keys are on this endpoint.
	// Match macmon's endpoint selection; retain the legacy service for Intel only.
	var last error
	for _, c := range candidates {
		if c.name != "AppleSMCKeysEndpoint" && !(runtime.GOARCH == "amd64" && c.name == "AppleSMC") {
			continue
		}
		var port uint32
		if code := smcAPI.open(c.entry, smcAPI.task(), 0, &port); code != 0 {
			last = fmt.Errorf("%s open: 0x%x", c.name, uint32(code))
			continue
		}
		return &smcConnection{port, c.name}, nil
	}
	if last != nil {
		return nil, last
	}
	return nil, fmt.Errorf("AppleSMC keys endpoint unavailable (%d services enumerated)", len(candidates))
}
func (s *smcConnection) close() { smcAPI.close(s.port) }
func (s *smcConnection) read(ctx context.Context, key string) (*float64, error) {
	return s.readDecoded(ctx, key, smcPowerValue)
}
func (s *smcConnection) readNumeric(ctx context.Context, key string) (*float64, error) {
	return s.readDecoded(ctx, key, smcNumericValue)
}
func (s *smcConnection) readDecoded(ctx context.Context, key string, decode func(string, []byte) *float64) (*float64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(key) != 4 {
		return nil, fmt.Errorf("invalid SMC key")
	}
	input := smcParam{Key: binary.BigEndian.Uint32([]byte(key)), Operation: 9}
	call := func() (smcParam, error) {
		var out smcParam
		size := unsafe.Sizeof(out)
		code := smcAPI.call(s.port, 2, unsafe.Pointer(&input), unsafe.Sizeof(input), unsafe.Pointer(&out), &size)
		if code != 0 {
			return out, fmt.Errorf("%s IOKit: 0x%x", key, uint32(code))
		}
		if out.Result != 0 {
			return out, fmt.Errorf("%s SMC status %d", key, out.Result)
		}
		if size != unsafe.Sizeof(out) {
			return out, fmt.Errorf("%s invalid response size %d", key, size)
		}
		return out, nil
	}
	out, err := call()
	if err != nil {
		return nil, err
	}
	info := out.Info
	if info.Size == 0 || info.Size > 32 {
		return nil, fmt.Errorf("%s invalid payload size %d", key, info.Size)
	}
	input.Info, input.Operation = info, 5
	out, err = call()
	if err != nil {
		return nil, err
	}
	var kind [4]byte
	binary.BigEndian.PutUint32(kind[:], info.Type)
	value := decode(string(kind[:]), out.Bytes[:info.Size])
	if value == nil {
		return nil, fmt.Errorf("%s missing/invalid value (type %q)", key, string(kind[:]))
	}
	return value, nil
}
func readSMCPower(ctx context.Context) CPUPowerReport {
	r := CPUPowerReport{At: time.Now()}
	s, err := openSMC()
	if err != nil {
		r.Problems = []string{err.Error()}
		return r
	}
	defer s.close()
	for _, key := range []string{"PCPC", "PCPT", "PCPR"} {
		value, err := s.read(ctx, key)
		if err != nil {
			r.Problems = append(r.Problems, err.Error())
			continue
		}
		r.Sensors = append(r.Sensors, CPUPower{ID: key, Name: "CPU package", Source: "AppleSMC / " + s.endpoint + " / " + key, Package: true, Watts: value})
		r.Problems = nil
		return r
	}
	return r
}
func readSMCThermal(ctx context.Context, model string) CPUThermalReport {
	r := CPUThermalReport{}
	s, err := openSMC()
	if err != nil {
		r.Problems = []string{err.Error()}
		return r
	}
	defer s.close()
	keys := appleCPUTemperatureKeys(model)
	var firstError error
	for _, key := range keys {
		value, err := s.read(ctx, key)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			continue
		}
		if !validTemperature(*value) {
			continue
		}
		r.Sensors = append(r.Sensors, CPUTemperature{ID: key, Name: "CPU sensor / " + key, Source: "AppleSMC / " + s.endpoint, Celsius: *value})
	}
	if len(r.Sensors) == 0 {
		r.Problems = append(r.Problems, fmt.Sprintf("No usable CPU temperature from %d model-matched SMC keys on %s.", len(keys), model))
		if firstError != nil {
			r.Problems = append(r.Problems, firstError.Error())
		}
	}
	return r
}
func platformCPUHardware(ctx context.Context, c *Collector) CPUHardware {
	if c.hardware.Power.At.IsZero() || time.Since(c.hardware.Power.At) >= 5*time.Second {
		c.hardware.Power = readSMCPower(ctx)
		if runtime.GOARCH == "arm64" && c.hardware.Power.PackageWatts() == nil {
			r := readIOReportCPUPower(ctx)
			if r.PackageWatts() == nil {
				r.Problems = append(r.Problems, c.hardware.Power.Problems...)
			}
			c.hardware.Power = r
		}
	}
	return c.hardware
}
