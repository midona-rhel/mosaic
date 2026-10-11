//go:build !windows

package metrics

import (
	"context"

	"github.com/shirou/gopsutil/v4/mem"
)

func platformMemoryExtra() (*MemoryExtra, error) { return nil, nil }

func platformSwap(ctx context.Context) (*mem.SwapMemoryStat, error) {
	return mem.SwapMemoryWithContext(ctx)
}
