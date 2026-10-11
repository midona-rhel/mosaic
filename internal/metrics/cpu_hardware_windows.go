//go:build windows

package metrics

import "context"

// Temperature, power and clocks share one bounded WMI query every five seconds.
func platformCPUHardware(_ context.Context, c *Collector) CPUHardware { return c.hardware }
