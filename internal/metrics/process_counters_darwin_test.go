//go:build darwin

package metrics

import (
	"context"
	"github.com/shirou/gopsutil/v4/process"
	"os"
	"testing"
)

// Native CI checks the ABI and time units against the established provider.
func TestDarwinCombinedProcessCounters(t *testing.T) {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before, err := p.TimesWithContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rss, memoryReady, total, cpuReady := sampleProcessCounters(ctx, p)
	after, err := p.TimesWithContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !memoryReady || rss == 0 || !cpuReady || total+1e-9 < before.User+before.System || total > after.User+after.System+1e-9 {
		t.Fatalf("bad task-info units or ABI: rss=%d cpu=%f before=%+v after=%+v", rss, total, before, after)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, memoryReady, _, cpuReady = sampleProcessCounters(cancelled, p)
	if memoryReady || cpuReady {
		t.Fatal("ignored cancellation")
	}
}
