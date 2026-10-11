package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var wddmAdapter = regexp.MustCompile(`luid_0x[0-9a-fA-F]+_0x[0-9a-fA-F]+_phys_[0-9]+`)
var wddmEngine = regexp.MustCompile(`eng_[0-9]+_engtype_[A-Za-z0-9_]+`)
var wddmLUID = regexp.MustCompile(`(?i)luid_0x([0-9a-f]+)_0x([0-9a-f]+)`)

type windowsAdapter struct {
	LUID, Name string
	Vendor     uint32
	Memory     uint64
	Software   bool
}

func adapterLUID(id string) string {
	parts := wddmLUID.FindStringSubmatch(id)
	if len(parts) != 3 {
		return ""
	}
	high, e1 := strconv.ParseUint(parts[1], 16, 32)
	low, e2 := strconv.ParseUint(parts[2], 16, 32)
	if e1 != nil || e2 != nil {
		return ""
	}
	return fmt.Sprintf("%08x:%08x", high, low)
}

func nameWindowsAdapters(r *GPUReport, adapters []windowsAdapter) {
	byID := map[string]windowsAdapter{}
	for _, a := range adapters {
		if id := adapterLUID(a.LUID); id != "" {
			byID[id] = a
		}
	}
	counts := map[string]int{}
	for _, g := range r.Devices {
		if g.Scope == "adapter counters" {
			counts[adapterLUID(g.ID)]++
		}
	}
	var devices []GPU
	for _, g := range r.Devices {
		a, ok := byID[adapterLUID(g.ID)]
		if ok && a.Software {
			continue
		}
		if ok {
			g.Name = a.Name
			g.Vendor = map[uint32]string{0x1002: "AMD", 0x10de: "NVIDIA", 0x8086: "Intel"}[a.Vendor]
			if counts[adapterLUID(g.ID)] == 1 && a.Memory > 0 {
				total := a.Memory
				g.MemoryTotal = &total
			}
			g.Note += " Adapter identity and dedicated capacity from DXGI LUID."
		}
		devices = append(devices, g)
	}
	r.Devices = devices
}

type windowsGPUInventory struct{ Name, PNPDeviceID string }
type windowsGPUEngine struct {
	Name                  string
	UtilizationPercentage *uint64
}
type windowsGPUMemory struct {
	Name                        string
	DedicatedUsage, SharedUsage *uint64
}
type windowsGPUReport struct {
	Inventory []windowsGPUInventory
	Engines   []windowsGPUEngine
	Memory    []windowsGPUMemory
	Problems  []string
}

func parseWindowsGPU(data []byte) (GPUReport, error) {
	var raw windowsGPUReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return GPUReport{}, fmt.Errorf("invalid Windows GPU JSON: %w", err)
	}
	r := GPUReport{Problems: raw.Problems}
	for _, v := range raw.Inventory {
		r.Devices = append(r.Devices, GPU{ID: v.PNPDeviceID, Name: v.Name, Source: "Windows CIM / Win32_VideoController", Scope: "inventory", Note: "Hardware inventory. WDDM LUID groups below are separate; no name-to-LUID mapping is assumed."})
	}
	groups := map[string]*GPU{}
	engines := map[string]map[string]float64{}
	group := func(id string) *GPU {
		if groups[id] == nil {
			groups[id] = &GPU{ID: id, Name: "WDDM adapter", Source: "Windows GPU performance counters / CIM", Scope: "adapter counters", Note: "Busiest physical engine after adding its process instances. LUID identifies the adapter in this boot session; dedicated/shared allocations are separate."}
		}
		return groups[id]
	}
	for _, v := range raw.Engines {
		id, engine := wddmAdapter.FindString(v.Name), wddmEngine.FindString(v.Name)
		if id == "" || engine == "" || v.UtilizationPercentage == nil {
			continue
		}
		value := float64(*v.UtilizationPercentage)
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		group(id)
		if engines[id] == nil {
			engines[id] = map[string]float64{}
		}
		engines[id][engine] += value
	}
	for id, values := range engines {
		busy := 0.0
		for _, v := range values {
			busy = max(busy, min(100, v))
		}
		group(id).Utilization = &busy
	}
	for _, v := range raw.Memory {
		id := wddmAdapter.FindString(v.Name)
		if id == "" {
			continue
		}
		g := group(id)
		g.MemoryUsed = v.DedicatedUsage
		g.SharedUsed = v.SharedUsage
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r.Devices = append(r.Devices, *groups[id])
	}
	if len(groups) == 0 && len(raw.Inventory) > 0 {
		r.Problems = append(r.Problems, "No WDDM adapter performance instances exposed by the driver")
	}
	for i := range r.Devices {
		r.Devices[i].ID = strings.TrimSpace(r.Devices[i].ID)
	}
	return r, nil
}
