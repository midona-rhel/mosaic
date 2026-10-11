package metrics

import (
	"encoding/binary"
	"math"
)

// AppleSMC ABI layout adapted from gopsutil v4.26.9 sensors_darwin.go.
// Copyright (c) 2014, WAKAYAMA Shirou. BSD-3-Clause; see THIRD_PARTY_NOTICES.
type smcVersion struct {
	Major, Minor, Build, Reserved byte
	Release                       uint16
}
type smcPLimit struct {
	Version, Length  uint16
	CPU, GPU, Memory uint32
}
type smcKeyInfo struct {
	Size, Type uint32
	Attributes byte
}
type smcParam struct {
	Key                       uint32
	Version                   smcVersion
	PLimit                    smcPLimit
	Info                      smcKeyInfo
	Result, Status, Operation byte
	Data                      uint32
	Bytes                     [32]byte
}

// SMC floating point values are native little-endian; fixed-point words are
// big-endian. Reject unknown formats instead of interpreting arbitrary bytes.
func smcPowerValue(kind string, data []byte) *float64 {
	if kind == "ui8 " {
		return nil
	} // Integer fan-count keys have no watt units.
	v := smcNumericValue(kind, data)
	if v == nil || !validPower(*v) || *v == 0 {
		return nil
	}
	return v
}
func smcNumericValue(kind string, data []byte) *float64 {
	var v float64
	switch {
	case kind == "ui8 " && len(data) == 1:
		v = float64(data[0])
	case kind == "flt " && len(data) == 4:
		v = float64(math.Float32frombits(binary.LittleEndian.Uint32(data)))
	case len(data) == 2 && len(kind) == 4 && (kind[:2] == "fp" || kind[:2] == "sp"):
		hex := func(b byte) int {
			if b >= '0' && b <= '9' {
				return int(b - '0')
			}
			if b >= 'a' && b <= 'f' {
				return int(b-'a') + 10
			}
			return -1
		}
		i, f := hex(kind[2]), hex(kind[3])
		bits := 16
		if kind[:2] == "sp" {
			bits = 15
		}
		if i < 0 || f < 0 || i+f != bits {
			return nil
		}
		n := int(binary.BigEndian.Uint16(data))
		if kind[:2] == "sp" {
			n = int(int16(n))
		}
		v = float64(n) / float64(uint64(1)<<f)
	default:
		return nil
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
