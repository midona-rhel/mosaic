package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/midona-rhel/mosaic/internal/metrics"
	"github.com/midona-rhel/mosaic/internal/sensors"
	"github.com/midona-rhel/mosaic/internal/ui"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mosaic:", err)
		os.Exit(1)
	}
}
func run() error {
	interval := flag.Duration("interval", time.Second, "Refresh delay (250ms to 10s; sampling time is additional)")
	demo := flag.Bool("demo", false, "Use clearly labeled demonstration data")
	asciiLogos := flag.Bool("ascii-logos", false, "Use plain ASCII for expanded OS logos too")
	noMouse := flag.Bool("no-mouse", false, "Disable mouse capture for terminal text selection")
	noTerminalEffects := flag.Bool("no-terminal-effects", false, "Disable pointer, cursor-guide, default background and title overrides")
	fira := flag.Bool("fira", false, "Use and remember joined meters (select Fira Code in your terminal)")
	meters := flag.String("meters", "", "Use and remember fira or portable meters (default: environment, saved preference, then portable)")
	readOnly := flag.Bool("read-only", false, "Disable process termination")
	snapshot := flag.Bool("snapshot", false, "Print one dashboard and exit")
	setupSensors := flag.Bool("setup-sensors", false, "Install or repair the Windows sensor driver (explicit UAC setup)")
	sensorsAdmin := flag.Bool("sensors-admin", false, "Launch Mosaic with Windows administrator sensor access")
	cpuDiagnostics := flag.Bool("cpu-diagnostics", false, "Print CPU-only sensor diagnostics as JSON and exit")
	gpuDiagnostics := flag.Bool("gpu-diagnostics", false, "Print GPU counters and temperature sources as JSON and exit")
	jsonOutput := flag.Bool("json", false, "Print one metrics snapshot as JSON and exit")
	color := flag.Bool("color", false, "Force true color, including in --snapshot output")
	width := flag.Int("width", 120, "Width for --snapshot (24 to 400)")
	height := flag.Int("height", 30, "Height for --snapshot (10 to 150)")
	panel := flag.String("panel", "", "Open a resource: system, cpu, memory, disks, processes, network, gpu, disk activity, battery, thermals, connections")
	layout := flag.String("layout", "", "Workspace file (default: user config directory/mosaic/layout.json)")
	noSave := flag.Bool("no-save-layout", false, "Start with default layout and do not save it")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("mosaic", version)
		return nil
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments; use --help")
	}
	if *fira {
		if *meters != "" && *meters != "fira" {
			return fmt.Errorf("--fira cannot be combined with --meters %s", *meters)
		}
		*meters = "fira"
	}
	switch *meters {
	case "":
	case "fira", "portable":
		if err := os.Setenv("UNICODE_PROGRESS_BAR", fmt.Sprint(*meters == "fira")); err != nil {
			return err
		}
	default:
		return fmt.Errorf("meters must be fira or portable")
	}
	if *interval < 250*time.Millisecond || *interval > 10*time.Second {
		return fmt.Errorf("interval must be between 250ms and 10s")
	}
	if *width < 24 || *width > 400 || *height < 10 || *height > 150 {
		return fmt.Errorf("snapshot size must be between 24×10 and 400×150")
	}
	modes := 0
	for _, mode := range []bool{*snapshot, *jsonOutput, *cpuDiagnostics, *gpuDiagnostics} {
		if mode {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("choose one of --snapshot, --json, --cpu-diagnostics or --gpu-diagnostics")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	defer metrics.CloseSensors()
	if *setupSensors {
		if modes > 0 || *sensorsAdmin || *demo {
			return fmt.Errorf("use --setup-sensors by itself")
		}
		return metrics.SetupSensors(ctx)
	}
	if *sensorsAdmin {
		launched, err := metrics.ElevateSensors()
		if err != nil || launched {
			return err
		}
	}
	model := ui.New(ctx, ui.Options{Interval: *interval, Demo: *demo, NoMouse: *noMouse, ReadOnly: *readOnly, NoTerminalEffects: *noTerminalEffects, ASCIIArt: *asciiLogos})
	if err := model.OpenPanel(*panel); err != nil {
		return err
	}
	if modes > 0 {
		s := metrics.Demo()
		if !*demo {
			c := metrics.NewCollector()
			if *cpuDiagnostics || *gpuDiagnostics {
				metrics.WaitSensors(ctx)
			}
			sample := func() metrics.Snapshot {
				sampleCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()
				return c.Sample(sampleCtx)
			}
			_ = sample() // Establish CPU and I/O baselines.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(*interval):
			}
			s = sample()
		}
		if *gpuDiagnostics {
			return writeJSON(struct {
				Build, OS, Architecture string
				GPU                     metrics.GPUReport
				SensorBackend           sensors.Report
			}{version, runtime.GOOS, runtime.GOARCH, s.GPU, metrics.SensorBackend()})
		}
		if *cpuDiagnostics {
			modelName, kernel := "", ""
			if len(s.CPUInfo) > 0 {
				modelName = s.CPUInfo[0].ModelName
			}
			if s.Host != nil {
				kernel = s.Host.KernelVersion
			}
			return writeJSON(struct {
				Build, OS, Architecture, Model, Kernel string
				Temperature                            metrics.CPUThermalReport
				Hardware                               metrics.CPUHardware
				CPUReady                               bool
				LogicalCPUPercent                      []float64
				Problems                               map[string]string
				SensorBackend                          sensors.Report
			}{version, runtime.GOOS, runtime.GOARCH, modelName, kernel, s.CPUThermal, s.CPUHardware, s.CPUReady, s.CPU, s.Problems, metrics.SensorBackend()})
		}
		if *jsonOutput {
			return writeJSON(s)
		}
		model.SetSize(*width, *height)
		model.SetSnapshot(s)
		v := model.View().Content
		if !*color {
			v = ansi.Strip(v)
		}
		fmt.Println(v)
		return nil
	}
	layoutPath := *layout
	if !*demo && !*noSave {
		if layoutPath == "" {
			dir, err := os.UserConfigDir()
			if err != nil {
				return err
			}
			layoutPath = filepath.Join(dir, "mosaic", "layout.json")
		}
		if err := model.LoadWorkspace(layoutPath); err != nil {
			return fmt.Errorf("load layout: %w (use --no-save-layout to bypass)", err)
		}
		if err := model.OpenPanel(*panel); err != nil {
			return err
		}
	}
	if *meters != "" {
		if err := model.SetMeterPreference(*meters); err != nil {
			return err
		}
	} else if os.Getenv("UNICODE_PROGRESS_BAR") == "" && model.MeterPreference() != "" {
		if err := os.Setenv("UNICODE_PROGRESS_BAR", fmt.Sprint(model.MeterPreference() == "fira")); err != nil {
			return err
		}
	}
	programOptions := []tea.ProgramOption{tea.WithContext(ctx), tea.WithFPS(60)}
	if *color {
		programOptions = append(programOptions, tea.WithColorProfile(colorprofile.TrueColor))
	}
	if !*noMouse && !*noTerminalEffects {
		programOptions = append(programOptions, tea.WithOutput(ui.PointerOutput(os.Stdout)))
	}
	_, err := tea.NewProgram(ui.WithMousePointer(model), programOptions...).Run()
	if !*demo && !*noSave && (err == nil || errors.Is(err, tea.ErrProgramKilled)) {
		if saveErr := model.SaveWorkspace(layoutPath); saveErr != nil {
			return fmt.Errorf("save layout: %w", saveErr)
		}
	}
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	return err
}

func writeJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
