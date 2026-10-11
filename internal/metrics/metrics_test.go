package metrics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

func TestCPUPercentExcludesGuestDoubleCounting(t *testing.T) {
	old := cpu.TimesStat{User: 10, Guest: 5, Idle: 50}
	now := cpu.TimesStat{User: 20, Guest: 10, Idle: 60}
	if got := CPUPercent(now, old); got != 50 {
		t.Fatalf("CPU=%f, expected 50%%", got)
	}
	if got := CPUPercent(old, now); got != 0 {
		t.Fatalf("reset yielded %f", got)
	}
	if got := Delta(1, 100, 1); got != 0 {
		t.Fatalf("reset underflow: %f", got)
	}
	if got := Delta(300, 100, 2); got != 100 {
		t.Fatalf("rate=%f", got)
	}
}
func TestWindowsInterruptTimeIsAlreadyInKernelTime(t *testing.T) {
	old := cpu.TimesStat{User: 10, System: 20, Idle: 70, Irq: 5}
	now := cpu.TimesStat{User: 20, System: 40, Idle: 100, Irq: 15}
	// Busy delta is 10 user + 20 kernel; total delta is 60. Adding Irq
	// again would produce 57.14% instead of 50%.
	if got := cpuPercentForOS(now, old, "windows"); got != 50 {
		t.Fatalf("Windows CPU interrupt double-counting: %f", got)
	}
	if got := cpuPercentForOS(now, old, "linux"); got <= 50 {
		t.Fatalf("Linux separate IRQ was dropped: %f", got)
	}
}
func TestIdentityGuardAndProtectedProcesses(t *testing.T) {
	ctx := context.Background()
	p, e := process.NewProcess(int32(os.Getpid()))
	if e != nil {
		t.Fatal(e)
	}
	started, e := p.CreateTimeWithContext(ctx)
	if e != nil {
		t.Skipf("process start time unavailable: %v", e)
	}
	target := Process{PID: p.Pid, Started: started + 1}
	if _, e = Inspect(ctx, target); e == nil {
		t.Fatal("PID reuse was not rejected")
	}
	target.Started = 0
	if _, e = Inspect(ctx, target); e == nil {
		t.Fatal("unknown identity was accepted")
	}
	target.Started = started
	if e = Terminate(ctx, target); e == nil {
		t.Fatal("self termination was accepted")
	}
	if e = Terminate(ctx, Process{PID: 1, Started: 1}); e == nil {
		t.Fatal("PID 1 was accepted")
	}
	if _, e = Inspect(ctx, target); e != nil {
		t.Fatalf("inspection of own process failed: %v", e)
	}
}
func TestLiveCollectorProducesPortableData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := NewCollector()
	first := c.Sample(ctx)
	if first.CPUReady {
		t.Fatal("first sample must establish baseline")
	}
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	second := c.Sample(ctx)
	if second.At.Before(first.At) {
		t.Fatal("sampling clock moved backwards")
	}
	if second.Host == nil || second.Memory == nil || len(second.Processes) == 0 || len(second.Volumes) == 0 {
		t.Fatalf("core live metrics missing: %+v", second.Problems)
	}
	if !second.CPUReady {
		t.Fatalf("second sample should include CPU deltas: %+v", second.Problems)
	}
	for _, v := range second.CPU {
		if v < 0 || v > 100 {
			t.Fatalf("CPU rate out of range: %f", v)
		}
	}
}
