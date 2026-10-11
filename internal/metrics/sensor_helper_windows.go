//go:build windows

package metrics

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/midona-rhel/mosaic/internal/sensors"
	"golang.org/x/sys/windows"
)

var sensorHelper = sensors.Bridge{Launch: func(ctx context.Context) (*exec.Cmd, error) {
	path, err := sensorHelperPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, path, "--stdio")
	configureGPUCommand(cmd)
	cmd.WaitDelay = 200 * time.Millisecond
	return cmd, nil
}}

func sensorHelperPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(exe), "sensors", "Mosaic.SensorHelper.exe")
	if _, err = os.Stat(path); err != nil {
		return "", fmt.Errorf("bundled helper missing; extract the complete Windows ZIP (including sensors)")
	}
	return path, nil
}
func CloseSensors() { sensorHelper.Close() }
func WaitSensors(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for {
		r := sensorHelper.Snapshot()
		if r.Protocol == 1 || (len(r.Problems) > 0 && r.Problems[0] != "Hardware sensors starting…") {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func SetupSensors(ctx context.Context) error {
	path, err := sensorHelperPath()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, path, "--setup")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("sensor setup: %w", err)
	}
	fmt.Println("Sensor driver setup finished. Run mosaic --sensors-admin for CPU temperature and power.")
	return nil
}

// Elevation is an explicit launch option, never a side effect of collecting data.
func ElevateSensors() (bool, error) {
	if windows.GetCurrentProcessToken().IsElevated() {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	var args []string
	for _, arg := range os.Args[1:] {
		args = append(args, syscall.EscapeArg(arg))
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	parameters, err := windows.UTF16PtrFromString(strings.Join(args, " "))
	if err != nil {
		return false, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return false, err
	}
	cwd, _ := windows.UTF16PtrFromString(dir)
	if err = windows.ShellExecute(0, verb, file, parameters, cwd, windows.SW_SHOWNORMAL); err != nil {
		return false, fmt.Errorf("sensor launch cancelled or failed: %w", err)
	}
	return true, nil
}

func SensorBackend() sensors.Report { return sensorHelper.Snapshot() }
