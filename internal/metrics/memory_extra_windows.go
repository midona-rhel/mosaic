//go:build windows

package metrics

import (
	"context"

	"github.com/shirou/gopsutil/v4/mem"
)

func platformMemoryExtra() (*MemoryExtra, error) {
	v, err := mem.NewExWindows().VirtualMemory()
	if err != nil {
		return nil, err
	}
	return &MemoryExtra{CommitUsed: v.CommitTotal, CommitLimit: v.CommitLimit}, nil
}

// Enumerate page files directly: no localized counter path and no estimate
// obtained by subtracting physical RAM from the system's commit limit.
func platformSwap(ctx context.Context) (*mem.SwapMemoryStat, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	devices, err := mem.SwapDevicesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	result := &mem.SwapMemoryStat{}
	for _, v := range devices {
		if v != nil {
			result.Used += v.UsedBytes
			result.Free += v.FreeBytes
		}
	}
	result.Total = result.Used + result.Free
	if result.Total > 0 {
		result.UsedPercent = 100 * float64(result.Used) / float64(result.Total)
	}
	return result, nil
}
