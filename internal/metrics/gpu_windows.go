//go:build windows

package metrics

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"syscall"
)

func configureGPUCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

// WMI class identifiers are locale-independent. Native COM queries avoid
// launching PowerShell for every telemetry refresh.
func windowsGPUCounters(ctx context.Context) ([]byte, error) {
	var r windowsGPUReport
	var err error
	r.Inventory, err = querySensors[windowsGPUInventory](ctx, "root/cimv2", "SELECT Name,PNPDeviceID FROM Win32_VideoController")
	if err != nil {
		r.Problems = append(r.Problems, "GPU inventory: "+err.Error())
	}
	r.Engines, err = querySensors[windowsGPUEngine](ctx, "root/cimv2", "SELECT Name,UtilizationPercentage FROM Win32_PerfFormattedData_GPUPerformanceCounters_GPUEngine")
	if err != nil {
		r.Problems = append(r.Problems, "GPU engine counters: "+err.Error())
	}
	r.Memory, err = querySensors[windowsGPUMemory](ctx, "root/cimv2", "SELECT Name,DedicatedUsage,SharedUsage FROM Win32_PerfFormattedData_GPUPerformanceCounters_GPUAdapterMemory")
	if err != nil {
		r.Problems = append(r.Problems, "GPU memory counters: "+err.Error())
	}
	return json.Marshal(r)
}

func platformGPU(ctx context.Context) GPUReport {
	r := GPUReport{}
	data, err := windowsGPUCounters(ctx)
	if err != nil {
		r.Problems = append(r.Problems, "Windows CIM: "+err.Error())
	} else {
		r, err = parseWindowsGPU(data)
		if err != nil {
			r.Problems = append(r.Problems, err.Error())
		}
	}
	if adapters, err := windowsAdapters(); err == nil {
		nameWindowsAdapters(&r, adapters)
	} else {
		r.Problems = append(r.Problems, "DXGI adapter identity: "+err.Error())
	}
	nvidia, err := nvidiaGPU(ctx)
	if err != nil {
		r.Problems = append(r.Problems, err.Error())
	}
	if len(nvidia) > 0 {
		filtered := r.Devices[:0]
		for _, g := range r.Devices {
			if g.Scope == "inventory" && strings.Contains(strings.ToLower(g.Name), "nvidia") {
				continue
			}
			filtered = append(filtered, g)
		}
		r.Devices = append(filtered, nvidia...)
	}
	return r
}
