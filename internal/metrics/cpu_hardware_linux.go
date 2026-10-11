//go:build linux

package metrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func platformCPUHardware(ctx context.Context, c *Collector) CPUHardware {
	return CPUHardware{Power: c.linuxCPUPower(ctx, "/sys"), Clocks: linuxCPUClocks(ctx, "/sys")}
}
func readUint(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}
func (c *Collector) linuxCPUPower(ctx context.Context, root string) CPUPowerReport {
	r := CPUPowerReport{At: time.Now()}
	next := map[string]energyCounter{}
	visited := map[string]bool{}
	var visit func(string, int)
	visit = func(path string, depth int) {
		if depth > 4 || ctx.Err() != nil {
			return
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil || visited[real] {
			return
		}
		visited[real] = true
		name := sensorText(filepath.Join(real, "name"))
		energyPath := filepath.Join(real, "energy_uj")
		if _, err := os.Stat(energyPath); err == nil {
			s := CPUPower{ID: real, Name: name, Source: energyPath, Package: strings.HasPrefix(name, "package-"), State: "Unavailable"}
			v, err := readUint(energyPath)
			if err != nil {
				r.Problems = append(r.Problems, fmt.Sprintf("%s: %v", name, err))
			} else {
				limit, _ := readUint(filepath.Join(real, "max_energy_range_uj"))
				current := energyCounter{v, limit, time.Now()}
				s.Watts, s.Interval = energyWatts(current, c.energy[real])
				s.State = "Sampling"
				if s.Watts != nil {
					s.State = ""
				}
				next[real] = current
			}
			limits, _ := filepath.Glob(filepath.Join(real, "constraint_*_power_limit_uw"))
			for _, p := range limits {
				v, err := readUint(p)
				if err != nil {
					continue
				}
				prefix := strings.TrimSuffix(p, "power_limit_uw")
				label := sensorText(prefix + "name")
				if label == "" {
					label = filepath.Base(prefix)
				}
				window, _ := readUint(prefix + "time_window_us")
				s.Limits = append(s.Limits, CPUPowerLimit{Name: label, Watts: float64(v) / 1e6, Window: time.Duration(window) * time.Microsecond})
			}
			r.Sensors = append(r.Sensors, s)
		}
		entries, err := os.ReadDir(real)
		if err != nil {
			r.Problems = append(r.Problems, err.Error())
			return
		}
		for _, entry := range entries {
			// Only walk powercap domains; never follow device/subsystem symlink cycles.
			if strings.Contains(entry.Name(), "rapl") {
				visit(filepath.Join(real, entry.Name()), depth+1)
			}
		}
	}
	visit(filepath.Join(root, "class", "powercap"), 0)
	visit(filepath.Join(root, "devices", "virtual", "powercap"), 0)
	c.energy = next
	sort.Slice(r.Sensors, func(i, j int) bool { return r.Sensors[i].ID < r.Sensors[j].ID })
	if ctx.Err() != nil {
		r.Problems = append(r.Problems, ctx.Err().Error())
	}
	if len(r.Sensors) == 0 {
		r.Problems = append(r.Problems, "No readable Linux powercap energy domains. RAPL support and energy_uj permissions depend on the CPU, kernel and administrator. Containers/VMs often expose none.")
	}
	return r
}
func linuxCPUClocks(ctx context.Context, root string) []CPUClock {
	var clocks []CPUClock
	paths, _ := filepath.Glob(filepath.Join(root, "devices/system/cpu/cpufreq/policy*"))
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		name := "cpuinfo_cur_freq"
		value, err := readUint(filepath.Join(p, name))
		kind := "Hardware reported"
		if err != nil || value == 0 {
			name = "scaling_cur_freq"
			value, err = readUint(filepath.Join(p, name))
			kind = "Driver reported (may be requested)"
		}
		if err != nil || value == 0 {
			continue
		}
		minimum, _ := readUint(filepath.Join(p, "scaling_min_freq"))
		maximum, _ := readUint(filepath.Join(p, "scaling_max_freq"))
		cpus := sensorText(filepath.Join(p, "affected_cpus"))
		clocks = append(clocks, CPUClock{Name: filepath.Base(p) + " / CPUs " + cpus, Source: filepath.Join(p, name), Kind: kind, MHz: float64(value) / 1000, MinMHz: float64(minimum) / 1000, MaxMHz: float64(maximum) / 1000, Governor: sensorText(filepath.Join(p, "scaling_governor"))})
	}
	return clocks
}
