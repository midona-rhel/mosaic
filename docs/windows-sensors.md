# Portable Windows hardware sensors

Extract the **whole Windows ZIP**, keeping `sensors` beside `mosaic.exe`, and run:

```powershell
.\mosaic.exe
```

The helper and its .NET runtime are included. Mosaic does not download or install
sensor drivers, register a service, or need a separate hardware-monitor
application for GPU readings. No setup command or installer is included.

The helper uses unmodified **LibreHardwareMonitorLib 0.9.6**, sampling in a separate
process every five seconds. It stays running between samples and communicates
through private stdin/stdout pipes. A slow or failed helper cannot stall the UI.
Missing, stale and failed readings remain unavailable. The helper never sets fan
controls, voltages or clock values.

## GPU readings

AMD temperature, hotspot, power, load and VRAM readings use the installed Radeon
driver's ADL interfaces, matching Libre Hardware Monitor's `AmdGpu` implementation.
No PawnIO installation is needed for that path. NVIDIA/Intel library readers and
existing WDDM/DXGI/NVIDIA readers remain available where supported by the driver.

Only unambiguous adapter names are merged. Identical devices are kept separate
rather than assigning one GPU's temperature to another.

## CPU readings and the portability limit

CPU utilization uses Windows counters and needs no sensor helper or driver.

Ryzen temperature and package power are different: Libre reads the CPU's SMN
registers and MSR energy counters using kernel access. Its current implementation
uses PawnIO. Mosaic can use an **already available, accessible** PawnIO driver, or
fall back to an already-running legacy LHM/OHM WMI sensor provider. It does not
install either. With no accessible provider, these readings remain unavailable.

If PawnIO is already present but access is denied, launch from an elevated
terminal, or explicitly request an elevated Mosaic window:

```powershell
.\mosaic.exe --sensors-admin
```

This uses Windows UAC and elevates Mosaic and its child helper for that session;
it installs nothing. Ordinary launches never request elevation.

Older Libre Hardware Monitor releases loaded WinRing0 temporarily. That driver
has known security issues and is blocked on many current Windows systems. Mosaic
does not bundle it or ask users to disable Windows protections. The current LHM
maintainers confirm that their new CPU backend is not fully portable without an
existing driver: [upstream discussion](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/issues/2140).

AMD ADLX's public system-metrics interfaces expose CPU usage and SmartShift power
distribution, not general Ryzen die-temperature/package-watt readings. These are
not substitutes for the missing sensor values.

## Diagnostics

```powershell
.\mosaic.exe --cpu-diagnostics
.\mosaic.exe --gpu-diagnostics
```

`SensorBackend` reports helper protocol, existing driver access, Windows error
code, sample time and raw named sensors. Error 5 means access was denied. If the
helper is missing, extract the complete ZIP again.

Native Windows tests cover self-contained startup, repeated samples, absence of
a CPU driver, collector integration and helper cleanup. Physical 5800X3D and
Radeon readings still need comparison on that machine. The helper is built for
Windows x64 and ARM64; sensor availability depends on hardware and driver APIs.

## Sources and packaging

- [Pinned library and integration API](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/tree/v0.9.6)
- [AMD CPU register/energy reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Cpu/Amd17Cpu.cs)
- [AMD GPU ADL reader](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Gpu/AmdGpu.cs)
- [Legacy WinRing0 loading and cleanup](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitorLib/Hardware/Ring0.cs)
- [PawnIO administrator/SYSTEM device ACL](https://github.com/namazso/PawnIO/blob/2.1.0/PawnIO/PawnIO.inf.in)
- [AMD ADLX public performance metrics](https://github.com/GPUOpen-LibrariesAndSDKs/ADLX/blob/main/SDK/Include/IPerformanceMonitoring.h)

The library is unmodified and licensed under MPL-2.0. Corresponding LHM source is
at the pinned tag (NuGet commit `3d331e3370efb858411f19511373eff65a218701`).
The ZIP includes licenses, runtime notices, dependency metadata/source URLs,
corresponding PawnIO.Modules 0.2.2 source and the NuGet lock file in
`sensors/licenses`. The embedded modules are used only with an existing driver.

Build with the .NET 10 SDK:

```powershell
python scripts/build_sensors.py --arch amd64 --output bin/sensors
```

Use `--arch arm64` for Windows ARM64. `scripts/release.py` builds both helpers
before packaging all six OS/architecture downloads. Bundling the runtime makes
the Windows ZIP larger, but users still extract and run one download.
