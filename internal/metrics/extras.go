package metrics

import (
	"context"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Battery struct {
	Name, State, Source                     string
	PowerSource, PowerProblem               string
	Percent, Health, Watts, Voltage, Cycles *float64
	Remaining, Full                         *float64
	CapacityUnit                            string
}
type BatteryReport struct {
	Devices []Battery
	Problem string
}
type Fan struct {
	Name, Source string
	RPM          float64
}
type FanReport struct {
	Devices []Fan
	Problem string
}
type Connection struct {
	Protocol, Local, Remote, State, Process string
	PID                                     int32
}
type ConnectionReport struct {
	Entries         []Connection
	Source, Problem string
}
type ExtraReport struct {
	Collected   ExtraOptions
	At          time.Time
	Battery     BatteryReport
	Fans        FanReport
	Connections ConnectionReport
}

// All slow extra providers share a ten-second cadence, run concurrently, and
// have independent deadlines. They never run in View or pointer handling.
type ExtraOptions struct{ Battery, Fans, Connections bool }

func collectExtras(ctx context.Context, wants ExtraOptions) ExtraReport {
	r := ExtraReport{Collected: wants}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if wants.Battery {
			r.Battery = platformBattery(c)
		}
	}()
	go func() {
		defer wg.Done()
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if wants.Fans {
			r.Fans = platformFans(c)
		}
	}()
	go func() {
		defer wg.Done()
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if wants.Connections {
			r.Connections = collectConnections(c)
		}
	}()
	wg.Wait()
	r.At = time.Now()
	return r
}
func collectConnections(ctx context.Context) ConnectionReport {
	r := ConnectionReport{Source: connectionSource()}
	rows, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		r.Problem = "Connection enumeration: " + err.Error()
		return r
	}
	names := map[int32]string{}
	seen := map[string]bool{}
	for _, v := range rows {
		if ctx.Err() != nil {
			r.Problem = "Partial results: collection deadline reached"
			break
		}
		proto := "TCP"
		if v.Type == 2 {
			proto = "UDP"
		}
		local := net.JoinHostPort(v.Laddr.IP, strconv.Itoa(int(v.Laddr.Port)))
		remote := "—"
		if v.Raddr.IP != "" && v.Raddr.Port != 0 {
			remote = net.JoinHostPort(v.Raddr.IP, strconv.Itoa(int(v.Raddr.Port)))
		}
		state := v.Status
		if proto == "UDP" {
			state = "BOUND"
		}
		key := fmt.Sprintf("%s/%s/%s/%s/%d", proto, local, remote, state, v.Pid)
		if seen[key] {
			continue
		}
		seen[key] = true
		name := "Unavailable"
		if v.Pid > 0 {
			if cached, ok := names[v.Pid]; ok {
				name = cached
			} else {
				if p, e := process.NewProcessWithContext(ctx, v.Pid); e == nil {
					if n, e := p.NameWithContext(ctx); e == nil && n != "" {
						name = n
					}
				}
				names[v.Pid] = name
			}
		}
		r.Entries = append(r.Entries, Connection{proto, local, remote, state, name, v.Pid})
	}
	sort.Slice(r.Entries, func(i, j int) bool {
		a, b := r.Entries[i], r.Entries[j]
		if (a.State == "LISTEN") != (b.State == "LISTEN") {
			return a.State == "LISTEN"
		}
		if a.Process != b.Process {
			return a.Process < b.Process
		}
		if a.Local != b.Local {
			return a.Local < b.Local
		}
		return a.Remote < b.Remote
	})
	return r
}
func nonnegative(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return nil
	}
	return &v
}
func ratio(a, b *float64) *float64 {
	if a == nil || b == nil || *b <= 0 {
		return nil
	}
	return nonnegative(100 * *a / *b)
}
func batteryState(charging, discharging, online bool) string {
	if charging {
		return "Charging"
	}
	if discharging {
		return "Discharging"
	}
	if online {
		return "On AC power"
	}
	return "Not charging"
}
