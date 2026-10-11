//go:build darwin

package metrics

import (
	"context"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/shirou/gopsutil/v4/process"
)

// The same libproc ABI and Mach time conversion used by gopsutil v4.26.9.
// One PROC_PIDTASKINFO call supplies both memory and CPU. Keep the library
// loaded rather than opening, binding and closing it twice per PID per sample.
var taskAPI struct {
	once           sync.Once
	err            error
	info           func(int32, int32, uint64, unsafe.Pointer, int32) int32
	timebase       func(unsafe.Pointer) int32
	secondsPerTick float64
}

func initTaskAPI() {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		taskAPI.err = err
		return
	}
	for _, f := range []struct {
		name   string
		target any
	}{{"proc_pidinfo", &taskAPI.info}, {"mach_timebase_info", &taskAPI.timebase}} {
		sym, err := purego.Dlsym(lib, f.name)
		if err != nil {
			taskAPI.err = err
			return
		}
		purego.RegisterFunc(f.target, sym)
	}
	var tb struct{ Numer, Denom uint32 }
	if taskAPI.timebase(unsafe.Pointer(&tb)) == 0 && tb.Denom > 0 {
		taskAPI.secondsPerTick = float64(tb.Numer) / float64(tb.Denom) / 1e9
	}
}

func sampleProcessCounters(ctx context.Context, p *process.Process) (uint64, bool, float64, bool) {
	if ctx.Err() != nil {
		return 0, false, 0, false
	}
	taskAPI.once.Do(initTaskAPI)
	if taskAPI.err != nil || taskAPI.secondsPerTick <= 0 {
		return 0, false, 0, false
	}
	var info process.ProcTaskInfo
	size := int32(unsafe.Sizeof(info))
	if taskAPI.info(p.Pid, 4 /* PROC_PIDTASKINFO */, 0, unsafe.Pointer(&info), size) != size {
		return 0, false, 0, false
	}
	total := (float64(info.Total_user) + float64(info.Total_system)) * taskAPI.secondsPerTick
	return uint64(info.Resident_size), true, total, true
}
