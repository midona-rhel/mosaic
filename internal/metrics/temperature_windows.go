//go:build windows

package metrics

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Read only existing WMI providers. Mosaic never starts a driver or elevates.
const windowsCPUThermalScript = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
$rows=@(); $problems=@()
foreach($ns in @('root/LibreHardwareMonitor','root/OpenHardwareMonitor')) {
 try {
  $hardware=@(Get-CimInstance -Namespace $ns -ClassName Hardware -Filter "HardwareType='Cpu'" | Select-Object Identifier,Name)
  $sensors=@(Get-CimInstance -Namespace $ns -ClassName Sensor -Filter "SensorType='Temperature' OR SensorType='Power' OR SensorType='Clock'" | Select-Object Identifier,Parent,Name,Value,SensorType)
  foreach($s in $sensors) {
   foreach($h in $hardware) {
    if($s.Parent -eq $h.Identifier) {
     $rows+=@{ID=$s.Identifier;Name=$s.Name;Hardware=$h.Name;Source=$ns;Value=$s.Value;Type=$s.SensorType;Parent=$s.Parent}
    }
   }
  }
  if($rows.Count -gt 0) { break }
 } catch { $problems+=($ns+': '+$_.Exception.Message) }
}
ConvertTo-Json -Depth 4 -Compress -InputObject @{Sensors=$rows;Problems=$problems}
`

func (c *Collector) platformCPUThermal(ctx context.Context) CPUThermalReport {
	c.hardware = CPUHardware{Power: CPUPowerReport{At: time.Now(), Problems: []string{"CPU power requires an accessible WMI hardware-monitor provider with CPU Power sensors enabled."}}}
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	data, err := gpuCommand(ctx, path, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsCPUThermalScript)
	if err != nil {
		return CPUThermalReport{Problems: []string{"CPU sensor provider: " + err.Error()}}
	}
	hardware, hardwareErr := parseWindowsCPUHardware(data)
	if hardwareErr == nil {
		c.hardware = hardware
		c.hardware.Power.At = time.Now()
	}
	r, err := parseWindowsCPUThermal(data)
	if err != nil {
		return CPUThermalReport{Problems: []string{err.Error()}}
	}
	if len(r.Sensors) == 0 {
		r.Problems = append(r.Problems, "Run a WMI-capable Libre Hardware Monitor or Open Hardware Monitor build with CPU sensors enabled. Windows ACPI thermal zones are not treated as CPU die temperature.")
	}
	return r
}
