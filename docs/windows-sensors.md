# Portable Windows hardware sensors

Extract the **whole Windows ZIP**, keeping `sensors` beside `mosaic.exe`, and run:

```powershell
.\mosaic.exe
```

The live Windows app requests administrator access automatically. Approve the
Windows UAC prompt to enable CPU temperature and power. There is one entry point:
`mosaic.exe`. When started from an elevated terminal it stays in that terminal.

For JSON/diagnostic commands, use an elevated terminal so their output stays
available to the calling shell. `--sensors-admin` remains a compatibility option
for explicitly requesting elevation. Demo/help/version commands do not elevate.

The helper, .NET runtime and signed sensor driver are included. No separate
installer, download or hardware-monitor application is needed. GPU driver APIs
work without elevation where the vendor permits it.

An elevated session can load the bundled signed PawnIO 2.1 driver using a
**temporary, demand-start kernel service**. This is driver loading, not a
driver-free implementation. It does not add an application installation,
Driver Store package or boot-start service. The last Mosaic reader normally
stops the driver and removes its service and private image on exit. Existing
third-party PawnIO services are borrowed and never changed or stopped.

The helper uses unmodified **LibreHardwareMonitorLib 0.9.6**, sampling every five
seconds in a collector child process. A host holds the driver session until the
collector exits, so native handles are released before unloading. Both stay
running between samples and communicate
through private stdin/stdout pipes. A slow or failed helper cannot stall the UI.
Missing, stale and failed readings remain unavailable. The helper never sets fan
controls, voltages or clock values.

## GPU readings

| Hardware | Reader | Portable readings and limits |
| --- | --- | --- |
| AMD GPUs | Radeon ADL | Load, VRAM, temperature/hotspot and power when exposed by the installed driver. |
| NVIDIA GPUs | NVAPI / NVML | Load, VRAM, temperature and power when exposed by the installed driver. |
| Intel discrete GPUs | Intel Graphics Control Library (IGCL) | Load, VRAM, temperature and power when exposed by the installed driver. |
| Intel integrated GPUs | D3D/WDDM, IGCL | Engine load and shared memory; temperature where IGCL exposes it. LHM's integrated-GPU power reader uses privileged MSR access. |

The GPU driver API paths need no additional PawnIO installation. Existing
WDDM/DXGI/NVIDIA readers remain available as fallbacks. This is source-verified
backend coverage, not physical testing of every adapter or driver version.

Only unambiguous adapter names are merged. Identical devices are kept separate
rather than assigning one GPU's temperature to another.

## CPU readings

CPU utilization uses Windows counters. AMD temperature/package power use Libre's
SMN and MSR readers; Intel uses its DTS and RAPL readers. Intel distance-to-TjMax
margins are excluded from actual temperatures.

The bundled session driver provides the kernel access those readers need.
No preinstalled PawnIO is required. Windows 10 2004 or newer is required for this
bundled backend. Driver loading requires elevation and remains subject to Windows
code integrity and organizational policy; Mosaic never disables either.
Hardware support still depends on the pinned Libre library. ARM64 driver loading
does not imply that Libre implements every ARM CPU sensor protocol.

Mosaic pins PawnIO **2.1.0** because it supports direct demand-start loading.
PawnIO 2.2 changed to Plug and Play registration. Both bundled 2.1 driver images
carry Microsoft hardware signatures; exact SHA-256 hashes are checked before
loading. The image is copied into a fresh directory accessible only to
administrators and SYSTEM. WinRing0 is not used.

Normal EOF shutdown closes the collector process before unloading. Active Mosaic readers hold process-scoped leases; only the last one requests
unloading. Pre-existing third-party drivers are never unloaded by Mosaic. If the helper is
forcibly killed, its demand-start service may remain until a subsequent Mosaic
session cleans it up or Windows restarts (restarting unloads the driver but can
leave metadata/files for the next session to remove).

AMD ADLX's public system-metrics interfaces expose CPU usage and SmartShift power
distribution, not general Ryzen die-temperature/package-watt readings. These are
not substitutes for the missing sensor values.

## Diagnostics

```powershell
.\mosaic.exe --cpu-diagnostics
.\mosaic.exe --gpu-diagnostics
```

`SensorBackend` reports helper protocol, driver mode/access, Windows error
code, sample time and raw named sensors. Error 5 means access was denied. If the
helper is missing, extract the complete ZIP again.

Native Windows tests exercise signature verification, temporary driver loading,
concurrent readers, cleanup, checksum rejection and collector integration. Physical AMD/Intel CPU and AMD/NVIDIA/Intel GPU readings still need comparison
on actual hardware. The helper is built for
Windows x64 and ARM64; sensor availability depends on hardware and driver APIs.

## Sources and packaging

- [Pinned library and integration API](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/tree/v0.9.6)
- [AMD CPU register/energy reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Cpu/Amd17Cpu.cs)
- [Intel CPU DTS/RAPL reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Cpu/IntelCpu.cs)
- [NVIDIA NVAPI/NVML reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Gpu/NvidiaGpu.cs)
- [Intel discrete GPU IGCL reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Gpu/IntelDiscreteGpu.cs)
- [Intel integrated GPU reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Gpu/IntelIntegratedGpu.cs)
- [AMD GPU ADL reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Gpu/AmdGpu.cs)
- [Legacy WinRing0 loading and cleanup](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitorLib/Hardware/Ring0.cs)
- [PawnIO administrator/SYSTEM device ACL](https://github.com/namazso/PawnIO/blob/2.1.0/PawnIO/PawnIO.inf.in)
- [AMD ADLX public performance metrics](https://github.com/GPUOpen-LibrariesAndSDKs/ADLX/blob/main/SDK/Include/IPerformanceMonitoring.h)

The library is unmodified and licensed under MPL-2.0. Corresponding LHM source is
at the pinned tag (NuGet commit `3d331e3370efb858411f19511373eff65a218701`).
The ZIP includes licenses, runtime notices, dependency metadata/source URLs,
corresponding PawnIO.Modules 0.2.2 source and the NuGet lock file in
`sensors/licenses`. The driver source (including PawnPP), license and linking exception are also
included. The modules run against either the bundled session driver or an
existing accessible PawnIO driver.

Build with the .NET 10 SDK:

```powershell
python scripts/build_sensors.py --arch amd64 --output bin/sensors
```

Use `--arch arm64` for Windows ARM64. `scripts/release.py` builds both helpers
before packaging all six OS/architecture downloads. Bundling the runtime makes
the Windows ZIP larger, but users still extract and run one download.
