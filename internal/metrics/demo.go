package metrics

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// Demo is explicitly selected, never used as a fallback for failed live data.
func Demo() Snapshot {
	at := time.Date(2026, 10, 10, 14, 32, 0, 0, time.UTC)
	s := Snapshot{
		At: at, Problems: map[string]string{}, RatesReady: true, DiskReady: true,
		Host:    &host.InfoStat{Hostname: "studio", OS: "darwin", Platform: "macOS", PlatformVersion: "26.0", KernelVersion: "25.0.0", KernelArch: "arm64", Uptime: 183840, BootTime: uint64(at.Unix()) - 183840, Procs: 6},
		CPUInfo: []cpu.InfoStat{{ModelName: "Apple M2 Pro", Cores: 10}}, CPU: []float64{22, 45, 16, 38, 12, 28, 96, 14, 76, 25}, CPUReady: true,
		Memory: &mem.VirtualMemoryStat{Total: 32 << 30, Used: 13 << 30, Available: 19 << 30, UsedPercent: 40.625, Free: 10 << 30, Inactive: 9 << 30, Active: 9 << 30, Wired: 4 << 30}, Swap: &mem.SwapMemoryStat{Total: 4 << 30, Used: 512 << 20, Free: 3584 << 20, UsedPercent: 12.5},
		Volumes:    []Volume{{"/dev/disk3s1", "/", "apfs", 1 << 40, 395 << 30, 629 << 30, 38.57}, {"/dev/disk4s1", "/Volumes/Archive", "apfs", 2 << 40, 1890 << 30, 158 << 30, 92.29}},
		Disks:      []IO{{"disk3", 2.4 * float64(1<<20), .8 * float64(1<<20)}},
		Interfaces: map[string]Interface{"en0": {MTU: 1500, Flags: "up|broadcast|running|multicast", HardwareAddress: "02:00:00:00:00:01", Addresses: []string{"192.0.2.12/24"}}, "lo0": {MTU: 16384, Flags: "up|loopback|running", Addresses: []string{"127.0.0.1/8", "::1/128"}}},
		Network:    []Traffic{{"en0", 8 << 30, 2 << 30, 824 * 1024, 146 * 1024, true}, {"lo0", 3 << 20, 3 << 20, 0, 0, true}},
		Processes:  []Process{{821, "firefox", 12.4, true, 1240 << 20, true, 1720000000000}, {1204, "code", 8.2, true, 864 << 20, true, 1720000010000}, {932, "docker", 4.6, true, 512 << 20, true, 1720000020000}, {2401, "mosaic", 1.2, true, 32 << 20, true, 1720000030000}, {510, "WindowServer", .8, true, 210 << 20, true, 1720000040000}, {1456, "terminal", .3, true, 94 << 20, true, 1720000050000}},
	}
	cpuPower := 24.6
	s.CPUHardware = CPUHardware{PhysicalCores: 10, LogicalCPUs: 10, Power: CPUPowerReport{At: at, Sensors: []CPUPower{{ID: "demo-package", Name: "CPU package (demo)", Source: "Demonstration fixture", Package: true, Watts: &cpuPower}}}}
	s.CPUThermal = CPUThermalReport{At: at, Sensors: []CPUTemperature{{ID: "demo-cpu-p", Name: "CPU performance cluster (demo)", Source: "Demonstration fixture", Celsius: 64.2}, {ID: "demo-cpu-e", Name: "CPU efficiency cluster (demo)", Source: "Demonstration fixture", Celsius: 51.8}}}
	u, temperature, power := 63.0, 54.0, 18.7
	s.GPU = GPUReport{At: at, Devices: []GPU{{ID: "DEMO-GPU", Name: "Apple M2 Pro graphics (demo)", Vendor: "Apple", Source: "Demonstration fixture", Scope: "accelerator", Utilization: &u, Temperature: &temperature, Power: &power, Note: "Illustrative values only. Unified memory belongs to system RAM; no dedicated VRAM capacity is invented."}}, Problems: []string{}}
	for i := range s.Processes {
		s.Processes[i].Started = at.Add(-time.Duration(i+1) * 45 * time.Minute).UnixMilli()
	}
	s.Extras = ExtraReport{At: at, Collected: ExtraOptions{true, true, true},
		Battery: BatteryReport{Devices: []Battery{{Name: "Internal battery (demo)", State: "Discharging", Source: "Demonstration fixture", Remaining: nonnegative(3666), Full: nonnegative(4700), CapacityUnit: "mAh", Percent: nonnegative(78), Health: nonnegative(94), Watts: nonnegative(12.8), Voltage: nonnegative(12.4), Cycles: nonnegative(143)}}},
		Fans:    FanReport{Devices: []Fan{{"Left fan (demo)", "Demonstration fixture", 1840}, {"Right fan (demo)", "Demonstration fixture", 1860}}},
		Connections: ConnectionReport{Source: "Demonstration fixture", Entries: []Connection{
			{"TCP", "127.0.0.1:3000", "—", "LISTEN", "node", 1210},
			{"TCP", "127.0.0.1:5432", "—", "LISTEN", "postgres", 1512},
			{"TCP", "192.0.2.12:53142", "203.0.113.5:443", "ESTABLISHED", "firefox", 821},
			{"UDP", "0.0.0.0:5353", "—", "BOUND", "mDNSResponder", 201},
		}},
	}
	return s
}
