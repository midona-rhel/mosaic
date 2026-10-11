# Telemetry sources and semantics

Audited on 10–11 October 2026. Scope is the three OS families requested for Mosaic: **Windows, macOS, and Linux**, on x64 and ARM64. BSD, Android, and other Go targets are not promised. Platform support does not imply that every GPU driver, virtual machine, container, protected process, or filesystem exports every field.

## Reference implementation

Mosaic pins [gopsutil v4.26.9](https://github.com/shirou/gopsutil/tree/v4.26.9), with the version and checksums recorded in `go.mod` and `go.sum`. Its source was inspected for each of the three OS backends, rather than assuming that similarly named fields have identical meanings. gopsutil is BSD-3-Clause and models the established [psutil](https://github.com/giampaolo/psutil) interfaces. The APIs used are `host.InfoWithContext`, `cpu.InfoWithContext`, `cpu.TimesWithContext`, `mem.VirtualMemoryWithContext`, `mem.SwapMemoryWithContext` (Unix), `mem.SwapDevicesWithContext` (Windows), `disk.PartitionsWithContext`, `disk.UsageWithContext`, `disk.IOCountersWithContext`, `net.IOCountersWithContext`, and the process inspection methods.

[bottom](https://github.com/ClementTsang/bottom/tree/7d65d62c8310f592727e6a142007f761504c2c8b/src/collection) is the secondary design reference for collection and presentation. Its cross-platform support is documented, and its GPU code separates NVIDIA, AMD, and Intel providers. It is evidence for using driver-specific collectors, not evidence that a universal GPU API exists. Mosaic implements its own small adapters; these are research references rather than vendored implementations.

## CPU

| Platform | Inspected reference implementation | Underlying counters |
| --- | --- | --- |
| Linux | [cpu_linux.go](https://github.com/shirou/gopsutil/blob/v4.26.9/cpu/cpu_linux.go) | Per-CPU `/proc/stat` cumulative ticks; `/proc/cpuinfo` for identity |
| macOS | [cpu_darwin.go](https://github.com/shirou/gopsutil/blob/v4.26.9/cpu/cpu_darwin.go) | Mach `host_processor_info(PROCESSOR_CPU_LOAD_INFO)`; `sysctl` hardware information |
| Windows | [cpu_windows.go](https://github.com/shirou/gopsutil/blob/v4.26.9/cpu/cpu_windows.go) | `NtQuerySystemInformation(SystemProcessorPerformanceInformation)` and processor-group handling; `GetSystemTimes` fallback; registry/SMBIOS/power APIs for identity |

Percentages come from **two cumulative samples**, never a cumulative boot-time fraction:

```text
CPU busy % = 100 × (Δtotal − Δidle) / Δtotal
Process CPU % = 100 × Δ(user_seconds + system_seconds) / Δwall_seconds
```

The Linux total includes user, system, nice, idle, iowait, irq, softirq, and steal. Guest and guest-nice are already included in user/nice and must not be added again. Mosaic treats idle+iowait as non-busy for this display. Linux explicitly cautions that iowait is not a precise per-CPU waiting measure and can decrease; negative idle differences are clamped. Counter resets or CPU identity changes re-establish the baseline.

On Windows, the pinned collector's `System` is kernel time minus idle, and already includes interrupt time. Its separate `Irq` field must **not** be added again. This audit corrected that double-counting, with a regression test that distinguishes Windows from Linux. `GetSystemTimes`' kernel value includes idle; gopsutil subtracts it before exposing System.

Total CPU is the average of logical CPU busy percentages on a 0–100 scale. A process uses **100% per occupied logical CPU**, so its percentage can exceed 100. Windows Task Manager's frequency-adjusted **Processor Utility** metric can differ from Mosaic's elapsed busy-time metric; a difference is not automatically an error. Container CPU readings can describe the host rather than the container's cgroup quota. Mosaic does not label these as quota-normalized.

Primary references:

- [Linux `/proc/stat` documentation](https://docs.kernel.org/filesystems/proc.html)
- [Apple `host_processor_info`](https://developer.apple.com/documentation/kernel/1502854-host_processor_info) and [XNU processor counter definitions](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/mach/processor_info.h)
- [Microsoft `GetSystemTimes`](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getsystemtimes) and [processor groups](https://learn.microsoft.com/en-us/windows/win32/procthread/processor-groups)
- [Microsoft CPU utility versus CPU time](https://learn.microsoft.com/en-us/troubleshoot/windows-client/performance/cpu-usage-exceeds-100)
- [Microsoft `GetProcessTimes`](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getprocesstimes)

## Physical memory, cache, commit, and swap

| Platform | Reference | Displayed meaning |
| --- | --- | --- |
| Linux | [mem_linux.go](https://github.com/shirou/gopsutil/blob/v4.26.9/mem/mem_linux.go) | `/proc/meminfo`; available is `MemAvailable`, or the library's documented fallback estimate; used = total − available |
| macOS | [mem_darwin.go](https://github.com/shirou/gopsutil/blob/v4.26.9/mem/mem_darwin.go) | Mach `host_statistics(HOST_VM_INFO)`, page size, `hw.memsize`; available = free + inactive pages; active/inactive/wired are shown separately |
| Windows | [mem_windows.go](https://github.com/shirou/gopsutil/blob/v4.26.9/mem/mem_windows.go) and [ex_windows.go](https://github.com/shirou/gopsutil/blob/v4.26.9/mem/ex_windows.go) | `GlobalMemoryStatusEx` physical total/available and `GetPerformanceInfo` system-wide committed bytes/commit limit |

Linux page cache is reclaimable to varying degrees; counting every cache byte as busy application memory is misleading. The UI shows the library's Linux cache field only on Linux. It no longer displays an unpopulated Windows/macOS cache field as `0 B`.

The macOS percentage is the pinned library's physical usage definition. It is **not Activity Monitor memory pressure**, and the details say so. Pressure includes compression, wired pages, reclaimability and swap activity; high occupancy alone does not prove bad pressure. The current public Mach wrapper does not expose all of those dimensions. Mosaic does not manufacture a pressure score from a usage percentage.

Windows `Free` in the common gopsutil interface equals physical available, including reclaimable/standby pages; it is not strictly untouched free pages. **Committed / limit** comes separately from `GetPerformanceInfo`. The pinned common swap implementation uses `\Paging File(_Total)\% Usage` and an estimate based on commit limit minus physical total. Its English performance-counter path can fail on localized installations. Mosaic therefore uses the same library's **SwapDevicesWithContext / EnumPageFilesW** instead: reported used and free pages from each pagefile are converted to bytes using the native page size and summed. This is displayed as **Pagefile**, separate from commit, and needs no English counter names. A failed enumeration stays unavailable.

Linux swap comes from swap totals/free in `/proc/meminfo`. macOS swap uses `vm.swapusage`. Zero swap is valid when the OS reports no allocated swap; a failed query is a different state.

Primary references:

- [Linux MemAvailable and memory fields](https://docs.kernel.org/filesystems/proc.html)
- [Apple Mach VM field definitions](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/mach/vm_statistics.h)
- [Apple Activity Monitor memory definitions](https://support.apple.com/guide/activity-monitor/view-memory-usage-actmntr1004/mac)
- [Microsoft memory performance information](https://learn.microsoft.com/en-us/windows/win32/memory/memory-performance-information)
- [Microsoft MEMORYSTATUSEX](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-memorystatusex) and [PERFORMANCE_INFORMATION](https://learn.microsoft.com/en-us/windows/win32/api/psapi/ns-psapi-performance_information)
- [Microsoft EnumPageFilesW](https://learn.microsoft.com/en-us/windows/win32/api/psapi/nf-psapi-enumpagefilesw) and [pagefile enumeration fields](https://learn.microsoft.com/en-us/windows/win32/api/psapi/ns-psapi-enum_page_file_information)

## Disk space and I/O

| Platform | Inspected source | Capacity / activity source |
| --- | --- | --- |
| Linux | [disk_linux.go](https://github.com/shirou/gopsutil/blob/v4.26.9/disk/disk_linux.go), [disk_unix.go](https://github.com/shirou/gopsutil/blob/v4.26.9/disk/disk_unix.go) | Mount enumeration and `statfs`; `/proc/diskstats` / block statistics |
| macOS | [disk_darwin.go](https://github.com/shirou/gopsutil/blob/v4.26.9/disk/disk_darwin.go), [disk_unix.go](https://github.com/shirou/gopsutil/blob/v4.26.9/disk/disk_unix.go) | `getfsstat`/`statfs`; whole IOMedia devices and IORegistry byte counters |
| Windows | [disk_windows.go](https://github.com/shirou/gopsutil/blob/v4.26.9/disk/disk_windows.go) | Logical drives, `GetDiskFreeSpaceEx`, fixed-volume `IOCTL_DISK_PERFORMANCE` |

Free space is **available to the current user**. On Unix, gopsutil uses `Bavail` for free and `(Blocks − Bfree)` for used; percentage is `used / (used + available)`. Reserved blocks mean used+available need not equal the raw total. Windows quotas can likewise affect space visible to the caller. The displayed percentage is the API's user-visible capacity percentage, not a recomputed ratio with a different denominator.

APFS volumes and snapshots can share a container; layered Linux filesystems and bind mounts can share storage. The app never sums mount capacities into a machine total. A container with no physical partition enumeration falls back to its root filesystem. Network mounts can be slow and some OS calls are not promptly cancellable; collection runs off the UI event loop.

Read/write rates are differences of cumulative byte counters divided by the **monotonic elapsed time between successful counter reads**. Linux disk sectors in the reference implementation are converted using the kernel-defined 512-byte statistic unit. A mount is not necessarily a physical device: Windows activity here is per fixed volume, macOS uses whole media devices, and Linux can include partition/virtual-device counters. The UI says **Volume / device I/O**. These are byte rates, not disk fullness, throughput capacity, or a cross-platform saturation metric.

Primary references:

- [Linux disk statistic definitions](https://docs.kernel.org/admin-guide/iostats.html)
- [Unix statfs fields](https://man7.org/linux/man-pages/man2/statfs.2.html)
- [Apple disk IOKit definitions](https://github.com/apple-oss-distributions/IOStorageFamily/blob/main/IOBlockStorageDriver.h)
- [Microsoft GetDiskFreeSpaceEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdiskfreespaceexw)
- [Microsoft DISK_PERFORMANCE](https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-disk_performance)

## Network

| Platform | Inspected source | Counters |
| --- | --- | --- |
| Linux | [net_linux.go](https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_linux.go) | `/proc/net/dev` per-interface bytes |
| macOS | [net_darwin.go](https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_darwin.go) | Routing sysctl interface records (`NET_RT_IFLIST2`) |
| Windows | [net_windows.go](https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_windows.go) | IP Helper `GetIfEntry2`, with older `GetIfEntry` fallback |

Rates use the last successful read timestamp, not an assumed one-second interval. First samples, newly discovered interfaces, and counter resets have an explicit warmup state. The overview excludes interfaces marked loopback by Go's OS-backed interface flags, with conventional names as a fallback. Per-interface details include them. Link flags, MTU, hardware addresses and assigned addresses are read through Go’s cross-platform `net.Interfaces` / `Interface.Addrs` APIs ([reference](https://pkg.go.dev/net#Interfaces)). These are read-only interface properties, not internet reachability or measured link capacity.

VPNs, bridges, tunnels and virtual NICs can count the same traffic more than once. The aggregate is not advertised as unique internet bytes. A high transfer rate is not automatically bad; network history uses blue/purple, with a clearly relative scale instead of a red saturation warning.

References: [Linux network counter notes](https://docs.kernel.org/filesystems/proc.html), [Apple routing/interface implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/net/route.h), and [Microsoft GetIfEntry2](https://learn.microsoft.com/en-us/windows/win32/api/netioapi/nf-netioapi-getifentry2).

## Processes and actions

| Platform | Inspected source | Collection |
| --- | --- | --- |
| Linux | [process_linux.go](https://github.com/shirou/gopsutil/blob/v4.26.9/process/process_linux.go) | `/proc/PID` stat/status/cmdline/exe, cumulative user/system time, RSS |
| macOS | [process_darwin.go](https://github.com/shirou/gopsutil/blob/v4.26.9/process/process_darwin.go) | BSD process sysctls and libproc `proc_pidinfo`/task data; process paths and resource usage where accessible |
| Windows | [process_windows.go](https://github.com/shirou/gopsutil/blob/v4.26.9/process/process_windows.go) | Process enumeration, Windows process handles, `GetProcessTimes`, `GetProcessMemoryInfo`, token/command information subject to access rights |

RSS means resident memory/working set; it is not unique private memory and cannot safely be summed to match physical RAM. Process CPU can exceed 100 because a process can run on multiple cores. Exited and inaccessible processes are expected during enumeration.

Some pinned macOS task-info methods do not validate the native call's returned byte count. Mosaic conservatively treats zero resident/task-time results there as unavailable instead of claiming that a protected process uses no resources. This also withholds legitimate zero-task measurements; it is documented rather than hiding the limitation. Thread and parent fields have availability flags, so failed queries do not display invented zero values.

Selection and actions use **PID plus creation time**. The start time is rechecked before inspection and termination. PID 1, the monitor itself, missing identity and detected PID reuse are refused. The check and signal are not atomic on every OS. Unix requests SIGTERM; Windows calls the OS termination API, which is not a promise of graceful application shutdown. Confirmation requires an explicit `y` or click; Enter is not affirmative. Demo mode touches no real process and `--read-only` disables termination.

References: [Linux process stat](https://man7.org/linux/man-pages/man5/proc_pid_stat.5.html), [Apple libproc API source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/proc_info.h), [Microsoft GetProcessTimes](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getprocesstimes), and [GetProcessMemoryInfo](https://learn.microsoft.com/en-us/windows/win32/api/psapi/nf-psapi-getprocessmemoryinfo).

## GPU: separate providers and explicit availability

Open the dedicated GPU panel with **g**, its dashboard card, or **7** in the default seven-panel workspace. The GPU view shows the provider, scope, sample age, identity, reported utilization and optional memory/temperature/power. Histories append only when a new GPU report arrives, not whenever cached data is redisplayed.

| OS / driver | Implemented source | Available fields and limits |
| --- | --- | --- |
| Linux / NVIDIA | Driver-supplied `nvidia-smi`, backed by NVML | UUID/name/PCI identity, busy-time utilization, dedicated used/total MiB converted to bytes, temperature, power; unsupported fields stay nil |
| Windows / NVIDIA | Same documented NVIDIA provider | Same optional device fields; WDDM restrictions can withhold metrics |
| Linux / AMD amdgpu | DRM device sysfs plus hwmon | `gpu_busy_percent`, `mem_info_vram_used/total`; readable `temp1_input` in millidegrees and `power1_average` in microwatts |
| Linux / Intel, Virtio, other DRM | DRM device inventory | PCI/vendor identity. Device-wide utilization is unavailable without a supported counter; the app does not extrapolate a global percentage from the processes it can see |
| Windows / WDDM 2.x AMD, Intel, NVIDIA | CIM GPU Engine and GPU Adapter Memory formatted performance classes | Per-LUID/physical-adapter busiest engine; dedicated allocation and shared allocation. Dedicated capacity and adapter names come from matching DXGI LUIDs |
| macOS / Apple, Intel, AMD accelerators | IOKit `IOAccelerator` registry enumeration via structured `ioreg -a` XML | Optional `Device Utilization %` or `GPU Activity(%)`; driver temperature with a model-matched AppleSMC fallback for M1–M5 integrated GPUs. These properties are driver-specific and undocumented; availability can change across hardware/macOS versions |

NVIDIA's utilization is the percentage of its driver sampling period with work executing, not memory occupancy. `utilization.memory` would mean memory-controller activity, not VRAM fullness, and is deliberately not substituted for used/total bytes. Unsupported `N/A` and `[Not Supported]` values remain absent. Driver polling is optional; failure does not remove CPU, RAM, processes, or disks.

Windows Task Manager's model takes the **busiest engine**, rather than averaging independent engines. Mosaic groups GPU Engine instances by LUID, physical adapter and engine number/type, sums process contributions to each engine, clamps each to 100, then takes the busiest. CIM class identifiers avoid localized English counter paths. Hardware names and dedicated capacities are matched through DXGI LUIDs; enumeration order is never used for the association. Its 32-bit `AdapterRAM` field is not used as VRAM capacity. NVIDIA device results and WDDM engine results can both be visible because they have different semantics; they are not summed into a machine GPU percentage.

macOS registry enumeration uses a public IOKit tool, but the utilization properties are **not a guaranteed public telemetry contract**. The established [Stats GPU reader](https://github.com/exelban/stats/blob/master/Modules/GPU/reader.swift) reads these same keys. Missing keys remain unavailable, including on Apple Silicon versions that only expose useful activity through private IOReport channels. Mosaic neither executes sudo/powermetrics automatically nor fabricates an IOReport reading. [macmon](https://github.com/vladkens/macmon) and [gptop's Apple backend](https://github.com/evilsocket/gptop/blob/main/docs/backends/apple.md) are researched alternatives that use private IOReport P-state residency; integrating them would require release-specific compatibility testing. Unified system RAM is never labeled a discrete GPU's dedicated VRAM.

Linux Intel DRM fdinfo exports **per-client** activity and sometimes engine capacity/cycles. The [bottom Intel collector](https://github.com/ClementTsang/bottom/blob/7d65d62c8310f592727e6a142007f761504c2c8b/src/collection/gpu/intel.rs) and kernel documentation show the identity/deduplication/delta requirements. Visible-process fdinfo can miss other users and protected processes; summing it and calling it whole-device GPU use would be misleading. That backend is not implemented in this version, and the UI says that utilization is unavailable.

GPU commands run without shell interpolation, without user profiles, with a three-second context deadline and a 2 MiB output bound. Windows CPU/GPU counters use native WMI/COM; optional driver tools use a hidden console. GPU reports are cached for five seconds and carry their own timestamp. OS/driver restrictions are reported instead of replacing live data with demo values.

Primary references:

- [NVIDIA nvidia-smi selective queries and utilization definitions](https://docs.nvidia.com/deploy/nvidia-smi/)
- [NVIDIA NVML reference](https://docs.nvidia.com/deploy/nvml-api/latest/index.html)
- [Linux amdgpu VRAM sysfs](https://docs.kernel.org/gpu/amdgpu/driver-misc.html)
- [Linux amdgpu busy / thermal / power interfaces](https://docs.kernel.org/gpu/amdgpu/thermal.html) and [hwmon unit definitions](https://docs.kernel.org/hwmon/sysfs-interface.html)
- [Linux DRM client usage statistics](https://docs.kernel.org/gpu/drm-usage-stats.html)
- [Microsoft GPUs in Task Manager: engines, memory, WDDM requirements](https://devblogs.microsoft.com/directx/gpus-in-the-task-manager/)
- [Microsoft performance counters](https://learn.microsoft.com/en-us/windows/win32/perfctrs/about-performance-counters)
- [Microsoft Win32_VideoController schema](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-videocontroller)
- [Apple IOKit matching/enumeration](https://developer.apple.com/documentation/iokit/1514494-ioservicegetmatchingservices)
- [Stats GPU reader](https://github.com/exelban/stats/blob/master/Modules/GPU/reader.swift)

## Verification and remaining gaps

Provider parser tests cover real schema shapes with **representative fixtures**, not claimed live hardware captures: unsupported NVIDIA values, MiB conversion, 64-bit Windows memory, busiest-engine aggregation, missing Apple keys, malformed output, sysfs device deduplication, AMD sensor units, cache behavior and cancellation. CPU tests cover Linux guest accounting and Windows interrupt inclusion. Live Linux collection and real terminal interaction are checked locally.

Native Windows and macOS runtime checks are configured in CI. PowerShell 5.1 and 7 launch tests and a native Windows ConPTY interaction test are included. Native Linux, Windows, and macOS suites passed for v0.1.0; physical GPU/sensor validation remains separate. Cross-compilation confirms build compatibility, not measurement correctness on those OSes or GPU hardware.

Before claiming fully validated support, record native results on Windows WDDM AMD/Intel/NVIDIA, Linux amdgpu/NVIDIA/Intel, Apple Silicon, and Intel Macs with discrete graphics. Compare equivalent definitions against documented OS tools under idle/load, permission restrictions, missing drivers and resize/mouse interactions. macOS IOReport activity, Intel whole-device GPU utilization, GPU processes, memory pressure, GPU clocks/fans and per-process disk I/O remain outside the implemented fields. These omissions are explicit; no universal library is advertised as filling them.

## CPU temperature

**Windows v0.1.5 update:** releases now bundle the direct LHM 0.9.6 sensor helper.
The WMI interface described below remains a legacy fallback. See
[Windows sensor setup, driver permissions and direct-library sources](windows-sensors.md).


Added after the feature-completeness audit identified this missing metric. Readings are not inferred from CPU utilization. `Snapshot.CPUThermal` retains sensor identity, label, provider, value, optional reported critical limit and control-value status. Sampling is cached for five seconds; the timestamp describes Mosaic’s provider query, not an independent guarantee that a driver has refreshed its hardware reading. A failed refresh replaces old readings with an unavailable report.

| Platform | Implementation and inspected reference | Limits |
| --- | --- | --- |
| Linux x64/ARM64 | Reads CPU hwmon `temp*_input`, `_label`, `_crit`, `_fault`, `_enable`; explicitly named CPU thermal zones as fallback. [Linux sysfs ABI](https://docs.kernel.org/hwmon/sysfs-interface.html), [coretemp](https://docs.kernel.org/hwmon/coretemp.html), [k10temp](https://docs.kernel.org/hwmon/k10temp.html), [k8temp](https://docs.kernel.org/hwmon/k8temp.html) | Drivers and readable sensors are required; VMs/containers often expose none. Generic ACPI, motherboard, NVMe and GPU sensors are excluded. |
| Intel macOS | Pinned [gopsutil SMC reader](https://github.com/shirou/gopsutil/blob/v4.26.9/sensors/sensors_darwin.go); selects CPU diode `TC0D` | Proximity/heatsink keys are not die readings. The upstream backend returns zero on failed reads, so zero is conservatively omitted. |
| Apple Silicon macOS | Pinned [gopsutil IOHID reader](https://github.com/shirou/gopsutil/blob/v4.26.9/sensors/sensors_darwin_arm64.go); explicitly CPU-named sensors and `pACC`/`eACC MTR Temp Sensor` families | Private/model-dependent interfaces; no guarantee on every macOS/Mac. The CPU/GPU/SoC distinction matches [Stats’ HID sensor map](https://github.com/exelban/stats/blob/master/Modules/Sensors/values.swift). |
| Windows x64/ARM64 | Bundled LHM 0.9.6 library helper; legacy `root/LibreHardwareMonitor` / `root/OpenHardwareMonitor` WMI fallback | CPU hardware access requires PawnIO and an elevated launch. No driver is installed; elevation is an explicit launch option. see [Windows setup](windows-sensors.md). ARM64 sensor/driver support depends on the device. |

The legacy WMI fallback was checked against Libre Hardware Monitor **v0.9.4** [Sensor.cs](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitor/WMI/Sensor.cs), [Hardware.cs](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitor/WMI/Hardware.cs) and [WmiProvider.cs](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitor/WMI/WmiProvider.cs). This specifies a WMI interface, not blanket compatibility with every later build. Newer builds that do not publish this interface will report unavailable. `Sensor.Max` is a historical maximum, not a thermal limit, and is deliberately not used as a critical threshold. These references are not vendored. The upstream WMI provider serializes null as zero; the adapter conservatively rejects zero/missing/non-finite/out-of-range readings.

Linux readings are converted from millidegrees Celsius. Recognized hwmon devices retain distinct paths so multi-socket/core sensors do not collapse into a single name. Faulted/disabled channels are omitted. Thermal fallback is considered when no usable CPU hwmon values exist, even if unrelated GPU/disk hwmon entries are present. CPU-only classification avoids silently presenting a motherboard or chassis temperature as CPU temperature.

The dashboard selects the hottest physical CPU reading. Tctl/control values are a separately labeled fallback only when no physical value is available; the k10temp documentation explicitly distinguishes Tdie and Tctl. A plausible range of −50…150°C rejects corrupt readings, and positive valid `_crit` readings are retained. Headline color uses the blue/purple/red ramp relative to a reported critical limit when one exists; otherwise it stays blue rather than inventing a model-specific danger threshold.

Tests cover units, sensor selection, fault flags, device-subdirectory layouts, GPU/ACPI exclusion, thermal fallback, AMD control semantics, Apple CPU/GPU/proximity separation, null/provider-zero handling, rendering at both standard sizes, pop-outs and unavailable explanations. Real temperature hardware on Windows/macOS has not been validated in this Linux environment.

## CPU power, clocks and processor details

`Snapshot.CPUHardware` carries physical/logical counts, live clock readings, and a separate power report. CPU package watts appear beside temperature in the CPU panel. GPU power remains separate. Neither value is whole-system wall power, battery discharge power, nor rated TDP. Missing readings are null/unavailable; the app does not calculate watts from CPU utilization. Power stays blue unless a defensible threshold exists; this version does not invent one.

| Platform | Implemented power source | Availability and semantics |
| --- | --- | --- |
| Linux x64/ARM64 | Read-only powercap RAPL `energy_uj` deltas divided by monotonic elapsed time, with class/device aliases deduplicated | Requires a kernel/CPU exposing readable RAPL domains. Package roots contribute to the headline; cores, uncore, DRAM and platform domains are listed separately and never added to packages. ARM devices without RAPL report unavailable. |
| Windows x64/ARM64 | Bundled LHM 0.9.6 CPU `Power` sensors; existing LHM/OHM WMI fallback | Direct helper samples in a separate process every five seconds with bounded I/O; the legacy WMI fallback has a two-second caller deadline. Direct helper requires PawnIO and administrator access; the optional elevated-launch flag is documented in windows-sensors.md. ARM64 support is provider-dependent. Package sensors are distinct from cores, graphics, memory and platform power. |
| Intel / Apple Silicon macOS | Native, read-only AppleSMC package keys `PCPC`, `PCPT`, `PCPR`, in that priority order | Alternative keys, never summed. The reader uses IOKit via purego and does not need CGO or launch sudo. Recognized keys depend on Mac model/macOS version; many Apple Silicon configurations may return unavailable. This is not an IOReport/powermetrics backend. |

Linux power samples are interval averages, not instantaneous measurements. The first sample, counter decrease (wrap or reset), changed counter range, and recovery after a failed read all rebaseline rather than fabricate spikes. Missing devices discard their baselines. Incomplete package readings never become a partial total. `constraint_*_power_limit_uw` and `constraint_*_time_window_us` are shown as **configured limits**, with their names and windows, never as actual consumption or rated TDP. This code never writes to powercap.

Windows power and clock records are filtered to CPU hardware by the native WMI query and decoded by SensorType. Provider null/zero power readings remain unavailable because the inspected WMI implementation serializes missing sensor values as zero. Linux's unchanged valid energy counter can legitimately yield 0 W. The macOS reader verifies the key type, payload size, IOKit result and SMC result; it decodes floating point/fixed point units and rejects unknown, non-finite, negative or all-zero payloads. Its ABI/read protocol is adapted from gopsutil under BSD-3-Clause; see `THIRD_PARTY_NOTICES`. It supports only documented-in-source key mappings, without assuming all Macs implement them. Native IOKit calls cannot be forcibly interrupted by a Go context; cancellation is checked between key reads.

Clock collection uses Linux cpufreq policy `cpuinfo_cur_freq` when readable and nonzero, otherwise `scaling_cur_freq`. The latter is explicitly labeled **driver reported (may be requested)**. Configured min/max frequency and governor remain separately labeled. Windows exposes CPU core Clock sensors from the same WMI query; bus clocks are excluded. There is no implemented live macOS clock backend. The CPU descriptor's `Mhz` is labeled **OS reference frequency**, because pinned gopsutil may return maximum/nominal frequency; it is never silently presented as the current clock. Physical/logical counts use `cpu.CountsWithContext` and describe the OS-visible topology, not a guessed CPU specification. Processor descriptors show available vendor, family/model/stepping, microcode and feature flags, deduplicated by package/model.

Inspected references:

- [Linux powercap documentation, RAPL energy and constraint semantics](https://docs.kernel.org/power/powercap/powercap.html), [maintained kernel source](https://github.com/torvalds/linux/blob/master/Documentation/power/powercap/powercap.rst).
- [Linux cpufreq policy attributes and current-frequency caveats](https://docs.kernel.org/admin-guide/pm/cpufreq.html).
- [Libre Hardware Monitor v0.9.4 Intel power/clock sensors](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitorLib/Hardware/Cpu/IntelCpu.cs) and [AMD 17h power/clock sensors](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.4/LibreHardwareMonitorLib/Hardware/Cpu/Amd17Cpu.cs).
- [Stats CPU power key map](https://github.com/exelban/stats/blob/5df24006b2993228fdf23f72fec467bffb4d2106/Modules/Sensors/values.swift) and [SMC units/read protocol](https://github.com/exelban/stats/blob/5df24006b2993228fdf23f72fec467bffb4d2106/SMC/smc.swift). Key mappings were researched; Stats implementation code was not copied.
- [gopsutil v4.26.9 SMC ABI/reader](https://github.com/shirou/gopsutil/blob/v4.26.9/sensors/sensors_darwin.go), [IOKit connection](https://github.com/shirou/gopsutil/blob/v4.26.9/internal/common/common_darwin.go), and [CPU implementations](https://github.com/shirou/gopsutil/tree/v4.26.9/cpu).

These implementations do not establish universal hardware support. Native Windows/macOS sensor validation, CPU fans/voltage, a portable throttling indicator, and whole-system/battery power remain unimplemented or unverified. The Apple Silicon IOReport follow-up below supersedes the earlier SMC-only power boundary.
## Apple Silicon CPU repair after local M5 feedback

The first local screenshot showed working CPU utilization on an Apple M5 Pro but unavailable temperature and package power. That exposed two implementation gaps, not a terminal-font issue. The prior Apple Silicon path used IOHID temperatures only; the SMC reader selected the first AppleSMC service and tried only three package-power keys.

Temperature now enumerates AppleSMC services and opens **AppleSMCKeysEndpoint**, following macmon. It probes explicit M1–M5 CPU temperature key maps selected by the CPU model, including the M5 `Tp00`…`Tp0y` set. Valid readings retain their SMC key, endpoint and Celsius value. IOHID remains a fallback. No broad `Tp*` probe is used: the same prefix/key can refer to a powerboard on another architecture. Missing keys and IOKit/SMC error codes are preserved when no sensor works.

Power retains the SMC package-key path, then uses a native, read-only **IOReport** adapter on Apple Silicon. It samples energy twice over approximately 250 ms and divides the energy delta by actual monotonic elapsed time. It understands mJ, uJ and nJ, rejects negative/error deltas, and labels the source and interval. Reports refresh every five seconds. Native handles are scoped to the sample and released on cancellation/error/success; libraries remain loaded for registered function pointers. There is no sudo, helper install, driver load or external macmon dependency.

On macOS 27, the adapter discovers the live AppleCLPC DriverID and bundle identifier, constructs the exact CPU energy channel from macmon's researched 17-driver catalog, and uses the subscription's accepted channel dictionary. Unsupported drivers are reported rather than assigned a guessed ID. Legacy Energy Model counters can freeze on macOS 27 without Apple's entitlement, so this build excludes that fallback on 27 and later; known CLPC mappings currently apply to major version 27 only. Earlier systems may use recognized Energy Model CPU totals/per-die counters. CLPC and legacy sources are never added together. Duplicate CLPC aliases are removed; missing/invalid components leave the package total unavailable; CPU totals and per-die components are not added together. Actual sample metadata determines identity, not array order.

Pinned references:

- [Stats' M1–M5 CPU sensor mappings](https://github.com/exelban/stats/blob/ac791a10a53b0457a467e7ee341b512ad9413e03/Modules/Sensors/values.swift).
- [macmon SMC endpoint and IOReport protocol](https://github.com/vladkens/macmon/blob/3f738177f6f87a08ba3d977540be6793b73863bc/src/sources.rs).
- [macmon CPU report-ID catalog](https://github.com/vladkens/macmon/blob/3f738177f6f87a08ba3d977540be6793b73863bc/src/clpc.rs) and [firmware extraction/verification method](https://github.com/vladkens/macmon/blob/3f738177f6f87a08ba3d977540be6793b73863bc/docs/clpc-discovery.md).
- [macmon source selection and macOS 27 frozen-counter caveat](https://github.com/vladkens/macmon/blob/3f738177f6f87a08ba3d977540be6793b73863bc/src/metrics.rs).

The macmon protocol/catalog adaptation is MIT-licensed and included in `THIRD_PARTY_NOTICES`. These are private, model/OS-dependent interfaces. Source research, parser tests and cross-builds do not establish native measurement accuracy; rerunning the new binary on the user's Mac is still required. Run `./mosaic --cpu-diagnostics` to print build, CPU model, kernel, temperature readings/errors and hardware power sources/errors. This output omits hostname, process lists, network addresses and disk inventory.


## Apple GPU audit and M5 temperature (UI fix 4)

The percentage is taken unchanged from `Device Utilization %`, falling back to `GPU Activity(%)` only when the first field is absent or invalid. This matches the pinned [Stats reader](https://github.com/exelban/stats/blob/ac791a10a53b0457a467e7ee341b512ad9413e03/Modules/GPU/reader.swift). The selected key is retained as `UtilizationCounter` and shown in details; the card says **Driver activity**. It is not percent of peak compute throughput, memory occupancy or a fabricated CPU-derived estimate. The driver's sampling window is unspecified, and Mosaic polls every five seconds. Display compositing, browsers and terminal drawing can produce activity without an obvious foreground GPU workload. A screenshot alone cannot validate the driver's accuracy. Compare contemporaneous Stats readings using the same counter; Activity Monitor or IOReport can have different sampling and aggregation semantics.

The previous build only used `Temperature(C)`, which was missing on the user's M5 Pro. The new fallback opens AppleSMCKeysEndpoint and reads the exact GPU keys from [Stats' pinned sensor map](https://github.com/exelban/stats/blob/ac791a10a53b0457a467e7ee341b512ad9413e03/Modules/Sensors/values.swift): M5 uses `Tg0U Tg0X Tg0d Tg0g Tg0j Tg1Y Tg1c Tg1g`. Separate verified M1–M4 mappings are included. There is no prefix scan, CPU-temperature substitution or proximity-temperature substitution. Available GPU die readings are averaged; individual keys and Celsius values appear in details/diagnostics. Invalid/zero/128 sentinel readings are rejected. A package temperature is assigned only when exactly one AGX accelerator is identified; multiple ambiguous devices are left unassigned. Missing readings remain visibly unavailable with a reason. This path is cross-built and fixture-tested, but needs native M5 verification.

`./mosaic --gpu-diagnostics` prints build identity and GPU-only JSON with raw selected utilization, counter name, temperature, source and individual sensor readings. It excludes process/network/disk inventory from its output.

## macOS process-query cost

The process collector now binds libSystem once and calls `proc_pidinfo(PROC_PIDTASKINFO)` once per PID for both resident memory and CPU time. It reuses gopsutil's exported `ProcTaskInfo` ABI and the same Mach timebase conversion, validates the returned structure size, and preserves unavailable flags on errors. The previous path called gopsutil MemoryInfo and Times separately; each opened and bound libSystem, queried the same task structure, then closed the library. Process names and creation times still use gopsutil; termination continues to revalidate creation time. Static CPU descriptors are cached for one minute; CPU utilization and live sensor clocks remain independently sampled. References: [pinned gopsutil process source](https://github.com/shirou/gopsutil/blob/v4.26.9/process/process_darwin.go) and [Apple libproc API](https://github.com/apple-oss-distributions/Libc/blob/main/include/libproc.h). A native Darwin comparison test is included for CI; local execution is unavailable here.

## Coverage checkpoint: universal hardware requirement

The current build is **not complete against a requirement for every CPU and GPU**. CPU load and system RAM use OS counters across Windows/macOS/Linux and do not depend on a short list of CPU models. CPU identity, topology, live clocks, temperature and watts have separate coverage and permission constraints described above. New or untested hardware is not assumed validated.

GPU coverage is narrower:

| Provider | GPU activity | GPU memory allocation | Remaining gap |
| --- | --- | --- | --- |
| NVIDIA on Linux/Windows | NVML via nvidia-smi, if exported | Dedicated used/total, if exported | Unsupported devices/driver modes and native hardware acceptance |
| AMD amdgpu on Linux | Firmware busy counter | VRAM used/total | Shared/APU accounting and missing sysfs counters |
| Windows WDDM 2.x | Busiest engine | Dedicated and shared bytes | Physical-driver acceptance and optional sensor coverage |
| macOS IOAccelerator | Optional driver counter | Not collected | Verified system-wide GPU memory provider; model/OS acceptance |
| Linux Intel/other DRM | Inventory only | Not collected | A supported activity/memory backend; honest scope for any per-client fdinfo aggregation |

The dashboard now displays dedicated allocation even without a capacity and shared allocation independently. It does not invent a memory percentage when the denominator is absent. Unknown GPU memory is explicitly unavailable rather than silently disappearing. Detailed views also state when power is unavailable. Apple unified system RAM must not be substituted for GPU allocation, and process-local Metal allocations must not be labeled system-wide GPU memory.

These are implementation gaps as well as hardware limitations; an unavailable label is not evidence that a missing collector has been completed. Windows/macOS runtime checks, multi-GPU identity and hardware comparisons remain necessary before claiming full support. Provider source links and exact semantics are in the GPU section above.

## Additional panels (panels-9)

Disk activity is now an independent panel; the default dashboard uses eight equal 1×1 cards. Battery, Thermals, and Connections can be added through `o`. Removing a panel is distinct from viewport overflow: removed cards stay removed until added, while hidden enabled cards return when there is room. Layout v2 preserves visibility alongside order, content, and spans; v1 files migrate without discarding user sizes.

| Panel | Linux | macOS (Intel / Apple Silicon) | Windows (x64 / ARM64) |
| --- | --- | --- | --- |
| Disk activity | gopsutil `/proc/diskstats` | gopsutil IOKit whole-media byte counters | gopsutil `IOCTL_DISK_PERFORMANCE`, per fixed volume |
| Battery | `/sys/class/power_supply`, system batteries only | `ioreg -r -c AppleSmartBattery -a`, parsed as XML | `root/wmi` BatteryStatus, BatteryFullChargedCapacity, BatteryStaticData, BatteryCycleCount; Win32_Battery charge/state fallback |
| Thermals / fans | Existing CPU/GPU readers plus hwmon `fan*_input` and labels | Existing CPU/GPU readers plus AppleSMC `FNum`, `F<n>Ac` | Existing CPU/GPU readers plus Fan sensors from an accessible Libre/Open Hardware Monitor WMI provider |
| Connections | gopsutil procfs socket/inode/PID mapping, current network namespace | gopsutil `lsof`, numeric TCP/UDP endpoints | gopsutil IP Helper TCP/UDP owner-PID tables |

Battery watts are measured at the battery, **not** whole-system or wall power. Linux micro-units are converted to watts/volts; health uses matching energy/energy or charge/charge capacity families. macOS follows Stats' distinction between percentage `MaxCapacity` and raw mAh capacities, and decodes signed discharge current before multiplying current by voltage. Windows joins battery WMI rows by InstanceName, rejects unknown sentinels, and withholds watts when capacity units are relative or unspecified. A desktop without a system battery reports no battery; a query failure is reported separately. Missing health, cycles, and watts remain unavailable.

The battery-fix-10 macOS reader additionally uses AppleSMC `PPBR` on Apple Silicon, following [Stats' battery-power selection](https://github.com/exelban/stats/blob/master/Modules/Battery/readers.swift). It falls back to `InstantAmperage × Voltage`, then `Amperage × Voltage`; signed current in 32-bit or 64-bit IORegistry representations is decoded before converting mA × mV to watts. [Apple's SmartBattery implementation](https://github.com/apple-oss-distributions/PowerManagement/blob/main/AppleSmartBatteryManager/AppleSmartBattery.cpp) distinguishes instantaneous and average current, exports InstantAmperage as a signed 32-bit value, and notes that zero average current can indicate a battery/error condition. A zero while explicitly discharging triggers the fallback and, if no usable reading exists, an unavailable value with an explanation. Zero remains valid on AC power. The selected power source is visible in details and JSON; no CPU load or adapter rating is converted to battery watts.

The UI calls the full-charge/design ratio **Capacity**, with its definition in details, instead of implying an OS health diagnosis. The JSON field `Health` is retained for compatibility. Values above 100% remain intact: they mean reported full-charge capacity exceeds nominal design capacity, not a charge above 100%. Hardware validation of PPBR/InstantAmperage on the user's M5/macOS 27 machine is still pending.

Fan values are read-only RPM, including a valid zero when stopped. Linux/Windows need an exposed driver/provider; Mac keys are hardware-dependent and fanless Macs have no fan counters. Thermals reuses identified CPU/GPU temperatures and preserves control-sensor labels. There is no fan-speed control or temperature inferred from CPU/GPU load.

Connections list accessible sockets, not a claim of complete system visibility. macOS may omit other users' sockets, Linux namespaces and process permissions constrain ownership, and Windows gopsutil may omit a socket family whose API call fails. Missing owners stay unavailable. Numeric addresses avoid DNS traffic; Mosaic neither probes hosts nor closes connections. The overview lists sockets (one process can own several); details retain PID, protocol, state, local and remote endpoints. Enumeration and name resolution are sampled together to avoid re-labeling a cached socket with a later process name.

Disk graphs retain independent per-device histories and separate read/write scales. The overview chooses the busiest reported device. Stacked devices/partitions and shared storage are never summed into a misleading machine total. Counter resets start from zero rate using the existing monotonic-delta guard.

The extra providers run concurrently off the UI path, use a ten-second cache and two-second provider contexts, and run only for enabled or opened panels. OS API cancellation support varies; native disk and Windows IP Helper calls are synchronous OS calls. Command providers have runtime/output bounds. CPU/GPU sensor cadence remains five seconds. JSON diagnostics deliberately request all extra providers, even when the dashboard defaults omit their cards.

Inspected references:

- [Linux power-supply ABI](https://github.com/torvalds/linux/blob/master/Documentation/ABI/testing/sysfs-class-power) and [hwmon sysfs interface](https://docs.kernel.org/hwmon/sysfs-interface.html): units, capacity families, system/peripheral scope, fan input and labels.
- [Stats battery reader](https://github.com/exelban/stats/blob/master/Modules/Battery/readers.swift): AppleRawMaxCapacity, NominalChargeCapacity, DesignCapacity, Amperage and Voltage semantics.
- [iStats SMC reader](https://github.com/Chris911/iStats/blob/master/ext/osx_stats/smc.c): fan count and RPM keys, fpe2 and float decoding. Mosaic reuses its existing IOKit ABI bridge; no iStats code was copied.
- [Microsoft Win32_Battery](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-battery); [battery driver structures and WMI layouts](https://github.com/reactos/reactos/blob/10cbc39abfc0903e1c1576edcf18da63584f7db8/sdk/include/psdk/batclass.h); [systeminformation's Windows battery reader](https://github.com/sebhildebrandt/systeminformation/blob/52dda715fe83d5f630d54778ad8e5b147c0de3c8/lib/battery.js) corroborates the capacity WMI classes.
- Pinned gopsutil [Linux socket reader](https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_linux.go), [macOS/Unix socket reader](https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_unix.go), and [Windows IP Helper reader](https://github.com/shirou/gopsutil/blob/v4.26.9/net/net_windows.go).

All three requested OS families have implementation paths. This does not establish that every motherboard, GPU, laptop firmware, or driver exposes every sensor. Native Mac/Windows hardware validation is still required; cross-compilation and fixtures cannot prove it.

Battery remaining/full capacity is exposed separately from current charge
percentage and the full/design ratio. macOS uses raw or nested mAh capacities
(and legacy Intel CurrentCapacity/MaxCapacity when they are absolute values).
Linux uses energy_now/energy_full in Wh, falling back to charge_now/charge_full
in mAh. Windows uses RemainingCapacity/FullChargedCapacity in Wh only when the
battery driver declares absolute capacity units; relative/unknown units remain
unavailable. Instantaneous voltage is not used to invent a Wh capacity. The
compact card shows remaining/full; the design ratio remains in details.

## Windows setup and adapter identity

CPU load uses Windows processor time counters. CPU die temperature and package
power use the bundled LHM 0.9.6 sensor helper with a bundled signed session driver and
administrator access. No separate installer or preinstalled driver is needed;
the live app requests elevation automatically and the helper uses a temporary demand-start service;
see [setup and diagnostics](windows-sensors.md). Older WMI providers remain a
fallback. Real 5800X3D readings still require hardware verification.

WDDM GPU engine and memory counters are queried through native WMI. DXGI adapter
LUIDs map counter groups to their actual AMD/NVIDIA/Intel device names and
64-bit dedicated-memory capacities. Software adapters are excluded when DXGI
identifies them. The dashboard shows the busiest measurable GPU; details retain
all reported adapters. Linked adapters with multiple physical nodes do not get
an invented per-node capacity. References: [DXGI_ADAPTER_DESC1](https://learn.microsoft.com/windows/win32/api/dxgi/ns-dxgi-dxgi_adapter_desc1)
and [Task Manager GPU metrics](https://devblogs.microsoft.com/directx/gpus-in-the-task-manager/).

Native CI covers API execution and fixtures, but its virtual hardware cannot
verify Ryzen or Radeon sensor accuracy. Compare diagnostics with readings from
the affected physical machine before treating a hardware issue as resolved.
