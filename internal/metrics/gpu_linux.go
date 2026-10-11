//go:build linux

package metrics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func platformGPU(ctx context.Context) GPUReport {
	r := GPUReport{}
	devices, err := linuxDRM("/sys/class/drm")
	if err != nil && !os.IsNotExist(err) {
		r.Problems = append(r.Problems, "DRM inventory: "+err.Error())
	}
	nvidia, err := nvidiaGPU(ctx)
	if err != nil {
		r.Problems = append(r.Problems, err.Error())
	}
	nvidiaPCI := map[string]bool{}
	for _, g := range nvidia {
		nvidiaPCI[g.PCI] = true
	}
	for _, g := range devices {
		// NVML has reliable identity and marketing names; don't duplicate its cards.
		if g.Vendor == "NVIDIA" && nvidiaPCI[normalizePCI(g.ID)] {
			continue
		}
		r.Devices = append(r.Devices, g)
	}
	r.Devices = append(r.Devices, nvidia...)
	return r
}

var drmCard = regexp.MustCompile(`^card[0-9]+$`)

func linuxDRM(root string) ([]GPU, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := []GPU{}
	seen := map[string]bool{}
	for _, e := range entries {
		if !drmCard.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(root, e.Name(), "device")
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || seen[resolved] {
			continue
		}
		seen[resolved] = true
		read := func(name string) string {
			data, _ := os.ReadFile(filepath.Join(path, name))
			return strings.TrimSpace(string(data))
		}
		vendor := map[string]string{"0x1002": "AMD", "0x8086": "Intel", "0x10de": "NVIDIA", "0x1af4": "Virtio"}[read("vendor")]
		if vendor == "" {
			vendor = "GPU"
		}
		g := GPU{ID: filepath.Base(resolved), Name: vendor + " " + read("device"), Vendor: vendor, Source: "Linux DRM sysfs", Scope: "device", Note: "Utilization is unavailable unless the driver exposes a documented device-wide counter."}
		if vendor == "AMD" {
			g.Source = "Linux amdgpu sysfs"
			g.Utilization = percentNumber(read("gpu_busy_percent"))
			g.MemoryUsed = uintNumber(read("mem_info_vram_used"))
			g.MemoryTotal = uintNumber(read("mem_info_vram_total"))
			if g.MemoryTotal != nil && *g.MemoryTotal == 0 {
				g.MemoryTotal, g.MemoryUsed = nil, nil
			}
			if g.MemoryUsed != nil && g.MemoryTotal != nil && *g.MemoryUsed > *g.MemoryTotal {
				g.MemoryUsed = nil
			}
			g.Note = "Firmware busy percentage; VRAM is dedicated memory, not shared system RAM."
			paths, _ := filepath.Glob(filepath.Join(path, "hwmon", "hwmon*"))
			for _, hw := range paths {
				if data, err := os.ReadFile(filepath.Join(hw, "temp1_input")); err == nil {
					if v := finiteNumber(string(data)); v != nil {
						t := *v / 1000
						g.Temperature = &t
					}
				}
				if data, err := os.ReadFile(filepath.Join(hw, "power1_average")); err == nil {
					if v := finiteNumber(string(data)); v != nil {
						p := *v / 1e6
						g.Power = &p
					}
				}
			}
		}
		result = append(result, g)
	}
	return result, nil
}
func uintNumber(s string) *uint64 {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return nil
	}
	return &v
}
