//go:build windows

package metrics

import "context"

func (c *Collector) enrichGPU(_ context.Context, r *GPUReport) {
	mergeHelperGPUs(r, helperGPUs(sensorHelper.Snapshot()))
}
