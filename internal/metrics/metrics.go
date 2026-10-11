// Package metrics collects portable system data independently of the UI.
package metrics

import (
	"context"
	"fmt"
	"math"
	stdnet "net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Process struct {
	PID         int32
	Name        string
	CPU         float64 // One fully occupied logical CPU is 100%.
	CPUReady    bool
	RSS         uint64
	MemoryReady bool
	Started     int64 // Milliseconds since epoch; also guards against PID reuse.
}

type Volume struct {
	Device, Mount, Type string
	Total, Used, Free   uint64
	Percent             float64
}
type Traffic struct {
	Name           string
	Received, Sent uint64
	Down, Up       float64
	Ready          bool
}
type Interface struct {
	MTU                    int
	Flags, HardwareAddress string
	Addresses              []string
}

type IO struct {
	Name        string
	Read, Write float64
}
type Snapshot struct {
	At          time.Time
	Host        *host.InfoStat
	CPUInfo     []cpu.InfoStat
	CPU         []float64
	CPUReady    bool
	RatesReady  bool
	Memory      *mem.VirtualMemoryStat
	Swap        *mem.SwapMemoryStat
	Volumes     []Volume
	Disks       []IO
	Network     []Traffic
	Processes   []Process
	Problems    map[string]string
	GPU         GPUReport
	CPUThermal  CPUThermalReport
	CPUHardware CPUHardware
	Loopback    map[string]bool
	Interfaces  map[string]Interface
	MemoryExtra *MemoryExtra
	Extras      ExtraReport
	DiskReady   bool
}

type procCounter struct {
	Started int64
	Total   float64
	At      time.Time
}

// Collector is owned by a single sampling command; calls must not overlap.
type Collector struct {
	netAt, diskAt              time.Time
	extras                     ExtraReport
	extraOptions               ExtraOptions
	cpuTimes                   []cpu.TimesStat
	netCounters                map[string]gnet.IOCountersStat
	diskCounters               map[string]disk.IOCountersStat
	processCounters            map[int32]procCounter
	gpu                        GPUReport
	thermal                    CPUThermalReport
	cpuModel                   string
	cpuInfo                    []cpu.InfoStat
	cpuInfoAt                  time.Time
	hardware                   CPUHardware
	energy                     map[string]energyCounter
	topologyAt                 time.Time
	physicalCores, logicalCPUs int
}

func NewCollector() *Collector { return &Collector{} }

func Delta(current, previous uint64, seconds float64) float64 {
	if seconds <= 0 || current < previous {
		return 0
	}
	return float64(current-previous) / seconds
}

func CPUPercent(current, previous cpu.TimesStat) float64 {
	return cpuPercentForOS(current, previous, runtime.GOOS)
}
func cpuTotal(t cpu.TimesStat, osName string) float64 {
	// Linux guest is included in User/Nice. On Windows gopsutil System is
	// Kernel-Idle and already includes InterruptTime, also exposed as Irq.
	total := t.User + t.System + t.Nice + t.Idle + t.Iowait + t.Softirq + t.Steal
	if osName != "windows" {
		total += t.Irq
	}
	return total
}
func cpuPercentForOS(current, previous cpu.TimesStat, osName string) float64 {
	d := cpuTotal(current, osName) - cpuTotal(previous, osName)
	idle := max(0, current.Idle+current.Iowait-previous.Idle-previous.Iowait)
	if d <= 0 {
		return 0
	}
	return math.Max(0, math.Min(100, 100*(d-idle)/d))
}

func (c *Collector) Sample(ctx context.Context) Snapshot {
	return c.SampleWithExtras(ctx, ExtraOptions{true, true, true})
}
func (c *Collector) SampleWithExtras(ctx context.Context, wants ExtraOptions) Snapshot {
	s := Snapshot{At: time.Now(), Problems: map[string]string{}, CPU: []float64{}, Volumes: []Volume{}, Disks: []IO{}, Network: []Traffic{}, Processes: []Process{}}
	var extras <-chan ExtraReport
	if c.extras.At.IsZero() || c.extraOptions != wants || time.Since(c.extras.At) >= 10*time.Second {
		ch := make(chan ExtraReport, 1)
		extras = ch
		go func() { ch <- collectExtras(ctx, wants) }()
	}
	s.Loopback = map[string]bool{}
	s.Interfaces = map[string]Interface{}
	if interfaces, err := stdnet.Interfaces(); err == nil {
		for _, iface := range interfaces {
			s.Loopback[iface.Name] = iface.Flags&stdnet.FlagLoopback != 0
			info := Interface{MTU: iface.MTU, Flags: iface.Flags.String(), HardwareAddress: iface.HardwareAddr.String()}
			if addrs, err := iface.Addrs(); err == nil {
				for _, addr := range addrs {
					info.Addresses = append(info.Addresses, addr.String())
				}
			}
			s.Interfaces[iface.Name] = info
		}
	} else {
		s.Problems["Network interfaces"] = err.Error()
	}
	problem := func(panel string, err error) {
		if err != nil {
			s.Problems[panel] = err.Error()
		}
	}
	var err error
	s.Host, err = host.InfoWithContext(ctx)
	problem("System", err)
	err = nil
	if len(c.cpuInfo) == 0 || time.Since(c.cpuInfoAt) >= time.Minute {
		c.cpuInfo, err = cpu.InfoWithContext(ctx)
		c.cpuInfoAt = time.Now()
	}
	s.CPUInfo = c.cpuInfo
	if len(s.CPUInfo) > 0 {
		c.cpuModel = s.CPUInfo[0].ModelName
	}
	problem("CPU", err)
	times, err := cpu.TimesWithContext(ctx, true)
	problem("CPU", err)
	if err == nil {
		s.CPUReady = len(times) > 0 && len(times) == len(c.cpuTimes)
		if s.CPUReady {
			for i, t := range times {
				if t.CPU != c.cpuTimes[i].CPU || cpuTotal(t, runtime.GOOS) <= cpuTotal(c.cpuTimes[i], runtime.GOOS) {
					s.CPUReady = false
					break
				}
			}
		}
		if s.CPUReady {
			for i, t := range times {
				s.CPU = append(s.CPU, CPUPercent(t, c.cpuTimes[i]))
			}
		}
		c.cpuTimes = times
	}
	s.Memory, err = mem.VirtualMemoryWithContext(ctx)
	problem("Memory", err)
	s.MemoryExtra, err = platformMemoryExtra()
	problem("Memory extras", err)
	s.Swap, err = platformSwap(ctx)
	problem("Swap", err)
	parts, err := disk.PartitionsWithContext(ctx, false)
	problem("Disks", err)
	// Containers can expose only an overlay root, excluded by physical partition
	// enumeration. Still report the filesystem containing the system root.
	if len(parts) == 0 {
		wd, _ := os.Getwd()
		root := filepath.VolumeName(wd) + string(os.PathSeparator)
		parts = append(parts, disk.PartitionStat{Device: "system volume", Mountpoint: root})
	}
	seen := map[string]bool{}
	for _, p := range parts {
		if ctx.Err() != nil {
			problem("Disks", ctx.Err())
			break
		}
		if seen[p.Mountpoint] {
			continue
		}
		seen[p.Mountpoint] = true
		u, e := disk.UsageWithContext(ctx, p.Mountpoint)
		if e != nil {
			problem("Disks", e)
			continue
		}
		s.Volumes = append(s.Volumes, Volume{p.Device, p.Mountpoint, p.Fstype, u.Total, u.Used, u.Free, u.UsedPercent})
	}
	dc, err := disk.IOCountersWithContext(ctx)
	problem("Disk I/O", err)
	if err == nil {
		now := time.Now()
		seconds := now.Sub(c.diskAt).Seconds()
		s.DiskReady = !c.diskAt.IsZero() && seconds > 0
		for name, v := range dc {
			if old, ok := c.diskCounters[name]; ok {
				s.Disks = append(s.Disks, IO{name, Delta(v.ReadBytes, old.ReadBytes, seconds), Delta(v.WriteBytes, old.WriteBytes, seconds)})
			}
		}
		c.diskCounters = dc
		c.diskAt = now
	}
	nc, err := gnet.IOCountersWithContext(ctx, true)
	problem("Network", err)
	if err == nil {
		now := time.Now()
		seconds := now.Sub(c.netAt).Seconds()
		s.RatesReady = !c.netAt.IsZero() && seconds > 0
		next := map[string]gnet.IOCountersStat{}
		for _, v := range nc {
			t := Traffic{Name: v.Name, Received: v.BytesRecv, Sent: v.BytesSent}
			if old, ok := c.netCounters[v.Name]; ok {
				t.Down = Delta(v.BytesRecv, old.BytesRecv, seconds)
				t.Up = Delta(v.BytesSent, old.BytesSent, seconds)
				t.Ready = seconds > 0 && v.BytesRecv >= old.BytesRecv && v.BytesSent >= old.BytesSent
			}
			s.Network = append(s.Network, t)
			next[v.Name] = v
		}
		c.netCounters = next
		c.netAt = now
	}
	ps, err := process.ProcessesWithContext(ctx)
	problem("Processes", err)
	nextProcs := map[int32]procCounter{}
	for _, p := range ps {
		if ctx.Err() != nil {
			problem("Processes", ctx.Err())
			break
		}
		name, e := p.NameWithContext(ctx)
		if e != nil {
			continue
		} // Process may have exited.
		v := Process{PID: p.Pid, Name: name}
		v.Started, _ = p.CreateTimeWithContext(ctx)
		rss, memoryReady, total, cpuReady := sampleProcessCounters(ctx, p)
		v.RSS, v.MemoryReady = rss, memoryReady
		if cpuReady {
			now := time.Now()
			if old, ok := c.processCounters[p.Pid]; ok && old.Started == v.Started && v.Started > 0 {
				dt := now.Sub(old.At).Seconds()
				if dt > 0 && total >= old.Total {
					v.CPU = 100 * (total - old.Total) / dt
					v.CPUReady = true
				}
			}
			nextProcs[p.Pid] = procCounter{v.Started, total, now}
		}
		s.Processes = append(s.Processes, v)
	}
	c.processCounters = nextProcs
	if err == nil && len(ps) > 0 && len(s.Processes) == 0 {
		s.Problems["Processes"] = "No accessible processes"
	}
	sort.Slice(s.Volumes, func(i, j int) bool { return s.Volumes[i].Mount < s.Volumes[j].Mount })
	sort.Slice(s.Disks, func(i, j int) bool { return s.Disks[i].Name < s.Disks[j].Name })
	sort.Slice(s.Network, func(i, j int) bool { return s.Network[i].Name < s.Network[j].Name })
	s.CPUThermal = c.sampleCPUThermal(ctx)
	s.CPUHardware = c.sampleCPUHardware(ctx)
	s.GPU = c.sampleGPU(ctx)
	if extras != nil {
		c.extras = <-extras
		c.extraOptions = wants
	}
	s.Extras = c.extras
	return s
}

type Detail struct {
	Process                           Process
	User, Executable, Command, Status string
	Threads                           int32
	Parent                            int32
	Problems                          []string
	ThreadsReady, ParentReady         bool
}

func Inspect(ctx context.Context, target Process) (Detail, error) {
	p, err := checkedProcess(ctx, target)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Process: target}
	field := func(label string, value string, err error) string {
		if err != nil {
			d.Problems = append(d.Problems, label+": "+err.Error())
			return "Unavailable"
		}
		return value
	}
	v, e := p.UsernameWithContext(ctx)
	d.User = field("User", v, e)
	v, e = p.ExeWithContext(ctx)
	d.Executable = field("Executable", v, e)
	v, e = p.CmdlineWithContext(ctx)
	d.Command = field("Command", v, e)
	status, e := p.StatusWithContext(ctx)
	d.Status = field("Status", strings.Join(status, ", "), e)
	d.Threads, e = p.NumThreadsWithContext(ctx)
	d.ThreadsReady = e == nil && d.Threads > 0
	if e != nil {
		d.Problems = append(d.Problems, "Threads: "+e.Error())
	}
	d.Parent, e = p.PpidWithContext(ctx)
	d.ParentReady = e == nil
	if e != nil {
		d.Problems = append(d.Problems, "Parent: "+e.Error())
	}
	return d, nil
}
func checkedProcess(ctx context.Context, target Process) (*process.Process, error) {
	if target.Started <= 0 {
		return nil, fmt.Errorf("process identity is unavailable; refresh and try again")
	}
	p, e := process.NewProcessWithContext(ctx, target.PID)
	if e != nil {
		return nil, e
	}
	started, e := p.CreateTimeWithContext(ctx)
	if e != nil {
		return nil, e
	}
	if started != target.Started {
		return nil, fmt.Errorf("process exited and its PID was reused; refresh before acting")
	}
	return p, nil
}
func Terminate(ctx context.Context, target Process) error {
	if target.PID <= 1 || target.PID == int32(os.Getpid()) {
		return fmt.Errorf("refusing to terminate PID %d", target.PID)
	}
	p, e := checkedProcess(ctx, target)
	if e != nil {
		return e
	}
	return p.TerminateWithContext(ctx)
}
