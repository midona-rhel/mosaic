//go:build windows

package metrics

import (
	"context"
	"testing"
	"time"
)

func TestNativeWindowsSensorInterfaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type osInfo struct{ Caption string }
	rows, err := querySensors[osInfo](ctx, "root/cimv2", "SELECT Caption FROM Win32_OperatingSystem")
	if err != nil || len(rows) != 1 || rows[0].Caption == "" {
		t.Fatalf("native WMI: %v %+v", err, rows)
	}
	if _, err := windowsAdapters(); err != nil {
		t.Fatalf("DXGI enumeration: %v", err)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := querySensors[osInfo](cancelled, "root/cimv2", "SELECT Caption FROM Win32_OperatingSystem"); err != context.Canceled {
		t.Fatalf("cancelled query: %v", err)
	}
}
