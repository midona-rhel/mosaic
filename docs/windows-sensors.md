# Windows hardware sensors

Extract the **whole Windows ZIP**, keeping `sensors` beside `mosaic.exe`. The
helper and its .NET runtime are included; no separate runtime or hardware-monitor
application is required.

Run normally for CPU utilization, memory, disks, processes and network:

```powershell
.\mosaic.exe
```

For CPU temperature and package power, install the driver once:

```powershell
.\mosaic.exe --setup-sensors
```

Setup downloads the unmodified PawnIO installer pinned to Libre Hardware Monitor
0.9.6 and verifies its SHA-256 before opening the Windows UAC installation prompt.
Internet access is needed for this setup step. Cancellation leaves normal
monitoring available. PawnIO can be removed through Windows Installed Apps.

Launch full sensor monitoring with:

```powershell
.\mosaic.exe --sensors-admin
```

This opens an elevated Mosaic window after UAC approval. PawnIO restricts device
access to administrators and SYSTEM, so **elevation is required for sensor reads,
not only for installation**. An already-elevated terminal can run `mosaic.exe`
directly. Normal launches never install a driver or request elevation. Full sensor
mode elevates Mosaic and its child helper; it does not install a background service.

The helper uses **LibreHardwareMonitorLib 0.9.6**, samples CPU/GPU sensors every
five seconds, and stays running between samples. It communicates over private
stdin/stdout pipes and exits with Mosaic. Collection runs off the UI thread; a
slow or failed helper cannot stall dragging. Missing, stale and failed readings
remain unavailable. The helper never sets fan controls, voltages or clock values.

GPU Core utilization, dedicated memory, core temperature, hotspot sensors and
package power are read when the library/vendor driver exposes them. GPU sensors
can work without PawnIO. Existing WDDM/DXGI and NVIDIA readers remain available.
Only unambiguous adapter names are merged; identical devices are kept separate
rather than assigning one GPU's temperature to another.

## Diagnostics

```powershell
.\mosaic.exe --cpu-diagnostics
.\mosaic.exe --gpu-diagnostics
```

`SensorBackend` reports helper protocol, driver installation/access, Windows
error code, sample time and raw named sensors. Error 5 means access was denied;
use an elevated terminal or `--sensors-admin`. If the helper is missing, extract
the complete ZIP again. Restart Mosaic after repairing or installing the driver.

Native Windows CI exercises self-contained startup, repeated samples, missing
driver behavior, collector integration and helper cleanup. The 5800X3D's actual
temperature/power accuracy still needs comparison on that machine. The helper is
built for Windows x64 and ARM64; exposed sensors and PawnIO/vendor-driver support
vary by processor and architecture. A successful build is not a claim that every
Windows ARM device exposes package power or temperature.

## Sources and packaging

- [Pinned library and integration API](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/tree/v0.9.6)
- [AMD Zen CPU sensors](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Cpu/Amd17Cpu.cs)
- [AMD GPU sensors](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Gpu/AmdGpu.cs)
- [PawnIO administrator/SYSTEM device ACL](https://github.com/namazso/PawnIO/blob/2.1.0/PawnIO/PawnIO.inf.in)
- [Pinned upstream installer](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitor/Resources/PawnIO_setup.exe)

Installer SHA-256: `a3a46226c5e2824f4cdd42be0eecbabfc672c86f7889710f5ab1e6ad385b47a0`.

The library is unmodified and licensed under MPL-2.0. Its corresponding source is
available at the pinned tag above (NuGet commit
`3d331e3370efb858411f19511373eff65a218701`). The Windows package includes the
library license, upstream third-party notices, .NET runtime notices, dependency
metadata/source URLs and NuGet lock file in `sensors/licenses`. PawnIO is obtained
from upstream during explicit setup, not redistributed in Mosaic's ZIP.

To build the complete helper from source, install the .NET 10 SDK and run:

```powershell
python scripts/build_sensors.py --arch amd64 --output bin/sensors
```

Use `--arch arm64` for Windows ARM64. `scripts/release.py` builds both helpers
before packaging all six OS/architecture downloads. Runtime files increase the
Windows ZIP size; users still extract and run one download.
