//go:build linux

package metrics

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (c *Collector) platformCPUThermal(ctx context.Context) CPUThermalReport {
	return linuxCPUThermal(ctx, "/sys")
}
func sensorText(path string) string { b, _ := os.ReadFile(path); return strings.TrimSpace(string(b)) }
func milliTemperature(path string) (float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	return v / 1000, err
}
func cpuThermalName(name string) bool {
	name = strings.ToLower(name)
	return name == "cpu" || name == "cpu-thermal" || name == "cpu_thermal" || name == "cpu-therm" || name == "x86_pkg_temp" || strings.HasPrefix(name, "cpu-thermal-") || strings.HasPrefix(name, "cpu_thermal-")
}
func linuxCPUThermal(ctx context.Context, root string) CPUThermalReport {
	r := CPUThermalReport{}
	dirs, _ := filepath.Glob(filepath.Join(root, "class/hwmon/hwmon*"))
	for _, dir := range dirs {
		if ctx.Err() != nil {
			r.Problems = append(r.Problems, ctx.Err().Error())
			break
		}
		// Some kernels put the hwmon attributes under device/.
		base := dir
		if sensorText(filepath.Join(base, "name")) == "" {
			base = filepath.Join(dir, "device")
		}
		driver := sensorText(filepath.Join(base, "name"))
		known := driver == "coretemp" || driver == "k10temp" || driver == "k8temp" || driver == "zenpower" || cpuThermalName(driver)
		if !known {
			continue
		} // Exclude ACPI zones, GPUs, disks and motherboard guesses.
		files, _ := filepath.Glob(filepath.Join(base, "temp*_input"))
		for _, file := range files {
			stem := strings.TrimSuffix(file, "_input")
			if sensorText(stem+"_fault") == "1" || sensorText(stem+"_enable") == "0" {
				continue
			}
			v, err := milliTemperature(file)
			if err != nil || !validTemperature(v) {
				r.Problems = append(r.Problems, "Cannot read CPU sensor "+file)
				continue
			}
			label := sensorText(stem + "_label")
			control := strings.Contains(strings.ToLower(label), "tctl") || ((driver == "k10temp" || driver == "zenpower") && label == "")
			if label == "" {
				label = driver + " " + filepath.Base(stem)
			}
			s := CPUTemperature{ID: file, Name: label + " (" + filepath.Base(dir) + ")", Source: "Linux hwmon / " + driver, Celsius: v, Control: control}
			if crit, err := milliTemperature(stem + "_crit"); err == nil && crit > 0 && validTemperature(crit) {
				s.Critical = &crit
			}
			r.Sensors = append(r.Sensors, s)
		}
	}
	// Avoid reporting aliases of the same CPU twice. Thermal zones are fallback
	// only, and only explicitly named CPU/package zones are admitted.
	if len(r.Sensors) == 0 {
		dirs, _ := filepath.Glob(filepath.Join(root, "class/thermal/thermal_zone*"))
		for _, dir := range dirs {
			if ctx.Err() != nil {
				break
			}
			name := sensorText(filepath.Join(dir, "type"))
			if !cpuThermalName(name) {
				continue
			}
			v, err := milliTemperature(filepath.Join(dir, "temp"))
			if err != nil || !validTemperature(v) {
				continue
			}
			r.Sensors = append(r.Sensors, CPUTemperature{ID: dir, Name: name + " (" + filepath.Base(dir) + ")", Source: "Linux thermal zone", Celsius: v})
		}
	}
	return r
}
