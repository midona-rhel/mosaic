//go:build !windows

package metrics

import (
	"context"
	"fmt"
	"github.com/midona-rhel/mosaic/internal/sensors"
)

func CloseSensors()               {}
func WaitSensors(context.Context) {}
func ElevateSensors() (bool, error) {
	return false, fmt.Errorf("--sensors-admin is a Windows launch option")
}

func SensorBackend() sensors.Report { return sensors.Report{} }
