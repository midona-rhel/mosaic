//go:build darwin

package metrics

import "context"

func platformGPU(ctx context.Context) GPUReport {
	data, err := gpuCommand(ctx, "/usr/sbin/ioreg", "-r", "-c", "IOAccelerator", "-a")
	if err != nil {
		return GPUReport{Problems: []string{"macOS IORegistry: " + err.Error()}}
	}
	devices, err := parseAppleGPU(data)
	if err != nil {
		return GPUReport{Problems: []string{err.Error()}}
	}
	return GPUReport{Devices: devices}
}
