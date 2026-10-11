package metrics

import "strings"

// Model-specific CPU temperature keys from Stats' sensor map (see telemetry.md).
// Never use a broad Tp* match: e.g. Tp0P is a powerboard sensor on Intel Macs.
func appleCPUTemperatureKeys(model string) []string {
	keys := []string{"TC0D", "TCAD"}
	for _, family := range []struct{ model, keys string }{
		{"Apple M1", "Tp09 Tp0T Tp01 Tp05 Tp0D Tp0H Tp0L Tp0P Tp0X Tp0b"},
		{"Apple M2", "Tp1h Tp1t Tp1p Tp1l Tp01 Tp05 Tp09 Tp0D Tp0X Tp0b Tp0f Tp0j"},
		{"Apple M3", "Te05 Te0L Te0P Te0S Tf04 Tf09 Tf0A Tf0B Tf0D Tf0E Tf44 Tf49 Tf4A Tf4B Tf4D Tf4E"},
		{"Apple M4", "Te05 Te0S Te09 Te0H Tp01 Tp05 Tp09 Tp0D Tp0V Tp0Y Tp0b Tp0e"},
		{"Apple M5", "Tp00 Tp04 Tp08 Tp0C Tp0G Tp0K Tp0O Tp0R Tp0U Tp0X Tp0a Tp0d Tp0g Tp0j Tp0m Tp0p Tp0u Tp0y"},
	} {
		if model == family.model || strings.HasPrefix(model, family.model+" ") {
			return append(keys, strings.Fields(family.keys)...)
		}
	}
	return keys
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
