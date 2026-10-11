//go:build windows

package metrics

import (
	"context"
	"os"
	"path/filepath"
)

func connectionSource() string { return "gopsutil / Windows IP Helper TCP/UDP owner-PID tables" }
func extraWindowsCommand(ctx context.Context, script string) ([]byte, error) {
	p := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	return gpuCommand(ctx, p, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
}

const windowsBatteryScript = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
$rows=@();$problem=''
try {
 $status=@(Get-CimInstance -Namespace root/wmi -ClassName BatteryStatus)
 $full=@(Get-CimInstance -Namespace root/wmi -ClassName BatteryFullChargedCapacity -ErrorAction SilentlyContinue)
 $design=@(Get-CimInstance -Namespace root/wmi -ClassName BatteryStaticData -ErrorAction SilentlyContinue)
 $cycles=@(Get-CimInstance -Namespace root/wmi -ClassName BatteryCycleCount -ErrorAction SilentlyContinue)
 foreach($b in $status) {
  if(-not $b.Active){continue}
  $f=$full | Where-Object InstanceName -eq $b.InstanceName | Select-Object -First 1
  $d=$design | Where-Object InstanceName -eq $b.InstanceName | Select-Object -First 1
  $c=$cycles | Where-Object InstanceName -eq $b.InstanceName | Select-Object -First 1
  $percent=$null;$rate=$null;$state='Not charging'
  if($f.FullChargedCapacity -gt 0 -and $f.FullChargedCapacity -ne 4294967295 -and $b.RemainingCapacity -ne 4294967295){$percent=100.0*$b.RemainingCapacity/$f.FullChargedCapacity}
  if($b.Charging){$state='Charging';$rate=$b.ChargeRate} elseif($b.Discharging){$state='Discharging';$rate=$b.DischargeRate} elseif($b.PowerOnline){$state='On AC power'}
  if($null -eq $d.Capabilities -or ($d.Capabilities -band 0x40000000) -ne 0){$rate=$null}
  $rows+=@{Name=$b.InstanceName;State=$state;Percent=$percent;Remaining=$b.RemainingCapacity;Full=$f.FullChargedCapacity;Capabilities=$d.Capabilities;Design=$d.DesignedCapacity;Rate=$rate;Voltage=$b.Voltage;Cycles=$c.CycleCount}
 }
} catch {$problem=$_.Exception.Message}
if($rows.Count -eq 0) {
 try {
  foreach($b in @(Get-CimInstance Win32_Battery)) {
   $state='Not charging';if($b.BatteryStatus -eq 1){$state='Discharging'}elseif($b.BatteryStatus -eq 2){$state='On AC power'}elseif($b.BatteryStatus -in @(6,7,8,9)){$state='Charging'}
   $rows+=@{Name=$b.Name;State=$state;Percent=$b.EstimatedChargeRemaining}
  }
  $problem=''
 } catch {$problem=$_.Exception.Message}
}
ConvertTo-Json -Depth 4 -Compress -InputObject @{Devices=$rows;Problem=$problem}
`
const windowsFansScript = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
$rows=@()
foreach($ns in @('root/LibreHardwareMonitor','root/OpenHardwareMonitor')) {
 try {
  foreach($s in @(Get-CimInstance -Namespace $ns -ClassName Sensor -Filter "SensorType='Fan'")) {
   if($null -ne $s.Value){$rows+=@{Name=$s.Name;Source=($ns+' / '+$s.Identifier);RPM=$s.Value}}
  }
  if($rows.Count -gt 0){break}
 } catch {}
}
$problem='';if($rows.Count -eq 0){$problem='Fan RPM requires an accessible Libre Hardware Monitor or Open Hardware Monitor WMI provider.'}
ConvertTo-Json -Depth 4 -Compress -InputObject @{Devices=$rows;Problem=$problem}
`

func platformBattery(ctx context.Context) BatteryReport {
	b, e := extraWindowsCommand(ctx, windowsBatteryScript)
	if e != nil {
		return BatteryReport{Problem: e.Error()}
	}
	return parseWindowsBattery(b)
}
func platformFans(ctx context.Context) FanReport {
	b, e := extraWindowsCommand(ctx, windowsFansScript)
	if e != nil {
		return FanReport{Problem: e.Error()}
	}
	return parseWindowsFans(b)
}
