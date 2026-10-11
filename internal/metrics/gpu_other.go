//go:build !linux && !darwin && !windows

package metrics

import "context"

func platformGPU(context.Context) GPUReport {
	return GPUReport{Problems: []string{"GPU collection is implemented for Linux, macOS and Windows"}}
}
