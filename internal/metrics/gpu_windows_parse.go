package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

var wddmAdapter = regexp.MustCompile(`luid_0x[0-9a-fA-F]+_0x[0-9a-fA-F]+_phys_[0-9]+`)
var wddmEngine = regexp.MustCompile(`eng_[0-9]+_engtype_[A-Za-z0-9_]+`)

func parseWindowsGPU(data []byte) (GPUReport, error) {
	var raw struct {
		Inventory []struct{ Name, PNPDeviceID string }
		Engines   []struct {
			Name                  string
			UtilizationPercentage *float64
		}
		Memory []struct {
			Name                        string
			DedicatedUsage, SharedUsage *uint64
		}
		Problems []string
	}
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
		value := *v.UtilizationPercentage
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
