//go:build !darwin && !windows

package metrics

import "context"

func (c *Collector) enrichGPU(_ context.Context, _ *GPUReport) {}
