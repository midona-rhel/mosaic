//go:build !darwin

package metrics

import (
	"context"
	"github.com/shirou/gopsutil/v4/process"
)

func sampleProcessCounters(ctx context.Context, p *process.Process) (rss uint64, memoryReady bool, total float64, cpuReady bool) {
	if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
		rss, memoryReady = mi.RSS, true
	}
	if t, err := p.TimesWithContext(ctx); err == nil && t != nil {
		total, cpuReady = t.User+t.System, true
	}
	return
}
