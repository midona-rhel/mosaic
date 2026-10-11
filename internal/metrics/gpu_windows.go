//go:build windows

package metrics

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func configureGPUCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

// CIM class identifiers are locale-independent, unlike English Get-Counter
// paths. Do not use Win32_VideoController.AdapterRAM: its uint32 truncates VRAM.
const windowsGPUScript = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
$inventory=@(Get-CimInstance Win32_VideoController | Select-Object Name,PNPDeviceID)
$engines=@();$memory=@();$problems=@()
try { $engines=@(Get-CimInstance Win32_PerfFormattedData_GPUPerformanceCounters_GPUEngine | Select-Object Name,UtilizationPercentage) } catch { $problems+='WDDM engine counters unavailable; a WDDM 2.x driver is required' }
try { $memory=@(Get-CimInstance Win32_PerfFormattedData_GPUPerformanceCounters_GPUAdapterMemory | Select-Object Name,DedicatedUsage,SharedUsage) } catch { $problems+='WDDM adapter memory counters unavailable' }
ConvertTo-Json -Depth 4 -Compress -InputObject @{Inventory=$inventory;Engines=$engines;Memory=$memory;Problems=$problems}
`

func platformGPU(ctx context.Context) GPUReport {
	r := GPUReport{}
	// Use the system copy; never depend on the user's chosen interactive shell.
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	data, err := gpuCommand(ctx, path, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsGPUScript)
	if err != nil {
		r.Problems = append(r.Problems, "Windows CIM: "+err.Error())
	} else {
		r, err = parseWindowsGPU(data)
		if err != nil {
			r.Problems = append(r.Problems, err.Error())
		}
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
