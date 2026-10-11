package metrics

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse the structured ioreg XML plist, never a localized pretty-printed table.
// Keys match the Stats GPU reader; these driver properties are undocumented
// and are deliberately optional, unlike the public IOKit enumeration API.
func parseAppleGPU(data []byte) ([]GPU, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var root any
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if s, ok := t.(xml.StartElement); ok && s.Name.Local == "array" {
			root, err = plistValue(d, s, 0)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	entries, ok := root.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid IORegistry plist array")
	}
	result := []GPU{}
	var visit func(map[string]any, bool)
	visit = func(entry map[string]any, root bool) {
		stats, _ := entry["PerformanceStatistics"].(map[string]any)
		if stats != nil || root {
			name, _ := entry["IORegistryEntryName"].(string)
			class, _ := entry["IOObjectClass"].(string)
			if name == "" {
				name = class
			}
			if name == "" {
				name = "macOS accelerator"
			}
			id := fmt.Sprint(entry["IORegistryEntryID"])
			g := GPU{ID: id, Name: name, Source: "macOS IOKit / IOAccelerator", Scope: "accelerator", Note: "Driver-reported activity over an unspecified driver sampling window, following Stats. This is not percent of peak compute throughput and is not averaged with CPU usage. Display compositing, browsers and terminal drawing can contribute. Unified RAM is not dedicated VRAM."}
			for _, key := range []string{"Device Utilization %", "GPU Activity(%)"} {
				if v, ok := stats[key]; ok {
					g.Utilization = percentNumber(fmt.Sprint(v))
					if g.Utilization != nil {
						g.UtilizationCounter = key
						break
					}
				}
			}
			if v, ok := stats["Temperature(C)"]; ok {
				g.Temperature = finiteNumber(fmt.Sprint(v))
				if g.Temperature != nil && (*g.Temperature <= 0 || *g.Temperature == 128 || !validTemperature(*g.Temperature)) {
					g.Temperature = nil
				}
				if g.Temperature != nil {
					g.TemperatureSource = "IOAccelerator / Temperature(C)"
				}
			}
			result = append(result, g)
		}
		children, _ := entry["IORegistryEntryChildren"].([]any)
		for _, child := range children {
			if m, ok := child.(map[string]any); ok {
				visit(m, false)
			}
		}
	}
	for _, entry := range entries {
		if m, ok := entry.(map[string]any); ok {
			visit(m, true)
		}
	}
	return result, nil
}
func plistValue(d *xml.Decoder, start xml.StartElement, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("IORegistry plist nesting exceeds limit")
	}
	if start.Name.Local == "dict" || start.Name.Local == "array" {
		m := map[string]any{}
		a := []any{}
		key := ""
		for {
			t, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch v := t.(type) {
			case xml.EndElement:
				if v.Name == start.Name {
					if start.Name.Local == "dict" {
						return m, nil
					}
					return a, nil
				}
			case xml.StartElement:
				x, err := plistValue(d, v, depth+1)
				if err != nil {
					return nil, err
				}
				if start.Name.Local == "array" {
					a = append(a, x)
				} else if v.Name.Local == "key" {
					key, _ = x.(string)
				} else {
					m[key] = x
				}
			}
		}
	}
	var value string
	if err := d.DecodeElement(&value, &start); err != nil {
		return nil, err
	}
	switch start.Name.Local {
	case "integer":
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "-") {
			return strconv.ParseInt(value, 10, 64)
		}
		base := 10
		if strings.HasPrefix(value, "0x") {
			base = 0
		}
		return strconv.ParseUint(value, base, 64)
	case "real":
		return strconv.ParseFloat(strings.TrimSpace(value), 64)
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return value, nil
	}
}
