package metrics

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Nil values mean unreported, never zero. Provider semantics are deliberately
// retained: NVML busy-time, AMD firmware busy, and WDDM busiest engine differ.
type GPU struct {
	ID, Name, Vendor, Source, Scope, Note, PCI             string
	Utilization, Temperature, Power                        *float64
	MemoryUsed, MemoryTotal, SharedUsed                    *uint64
	UtilizationCounter, TemperatureSource, TemperatureNote string
	TemperatureSensors                                     []GPUTemperatureSensor
}

type GPUTemperatureSensor struct {
	Key     string
	Celsius float64
}

type GPUReport struct {
	At       time.Time
	Devices  []GPU
	Problems []string
}

// GPU tools are slower than kernel counters. Cache separately, label sample age,
// and place a hard bound on command runtime and output size.
func (c *Collector) sampleGPU(ctx context.Context) GPUReport {
	if !c.gpu.At.IsZero() && time.Since(c.gpu.At) < 5*time.Second {
		return c.gpu
	}
	if ctx.Err() != nil {
		return GPUReport{Problems: []string{ctx.Err().Error()}}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	r := platformGPU(ctx)
	c.enrichGPU(ctx, &r)
	r.At = time.Now()
	if r.Devices == nil {
		r.Devices = []GPU{}
	}
	if r.Problems == nil {
		r.Problems = []string{}
	}
	if len(r.Devices) == 0 && len(r.Problems) == 0 {
		r.Problems = append(r.Problems, "No GPU exposed by the OS or driver (common in VMs and containers)")
	}
	c.gpu = r
	return r
}

type limitedOutput struct{ data []byte }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > 2<<20 {
		return 0, fmt.Errorf("collector output exceeds 2 MiB")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func gpuCommand(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	configureGPUCommand(cmd)
	cmd.WaitDelay = 200 * time.Millisecond
	var out limitedOutput
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("collector command failed: %w", err)
	}
	return out.data, nil
}

func nvidiaGPU(ctx context.Context) ([]GPU, error) {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil, nil
	} // Optional driver-supplied provider.
	data, err := gpuCommand(ctx, path, "--query-gpu=uuid,name,pci.bus_id,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	if err != nil {
		return nil, err
	}
	return parseNVIDIA(string(data))
}
func finiteNumber(s string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return nil
	}
	return &v
}
func percentNumber(s string) *float64 {
	v := finiteNumber(s)
	if v != nil && *v > 100 {
		return nil
	}
	return v
}
func mibNumber(s string) *uint64 {
	v := finiteNumber(s)
	if v == nil || *v >= float64(^uint64(0))/(1<<20) {
		return nil
	}
	u := uint64(*v * (1 << 20))
	return &u
}
func parseNVIDIA(data string) ([]GPU, error) {
	reader := csv.NewReader(strings.NewReader(data))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = 8
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid nvidia-smi CSV: %w", err)
	}
	result := make([]GPU, 0, len(rows))
	for _, row := range rows {
		g := GPU{ID: strings.TrimSpace(row[0]), Name: strings.TrimSpace(row[1]), Vendor: "NVIDIA", Source: "NVIDIA NVML / nvidia-smi", Scope: "device", Note: "Busy-time over the driver's sampling window; not a WDDM engine average."}
		g.PCI = normalizePCI(row[2])
		if g.ID == "" || g.Name == "" {
			return nil, fmt.Errorf("missing NVIDIA device identity")
		}
		g.Utilization = percentNumber(row[3])
		g.MemoryUsed = mibNumber(row[4])
		g.MemoryTotal = mibNumber(row[5])
		g.Temperature = finiteNumber(row[6])
		g.Power = finiteNumber(row[7])
		if g.MemoryTotal != nil && g.MemoryUsed != nil && *g.MemoryUsed > *g.MemoryTotal {
			g.MemoryUsed = nil
		}
		g.Note += " PCI " + strings.TrimSpace(row[2])
		result = append(result, g)
	}
	return result, nil
}

func normalizePCI(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	parts := strings.Split(s, ":")
	if len(parts) == 3 && len(parts[0]) > 4 {
		parts[0] = parts[0][len(parts[0])-4:]
	}
	return strings.Join(parts, ":")
}
