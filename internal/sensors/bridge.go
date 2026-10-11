// Package sensors runs the optional bundled hardware helper outside the UI and
// collector threads. It keeps one process warm and never installs a driver.
package sensors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type Sensor struct {
	ID, Parent, DeviceID, Hardware, HardwareType, Name, Type, Source string
	Value                                                            *float64
}
type Report struct {
	Protocol                          int
	At                                time.Time
	DriverInstalled, DriverAccessible bool
	DriverVersion                     string
	DriverError                       int
	Sensors                           []Sensor
	Problems                          []string
}

type Bridge struct {
	mu      sync.Mutex
	report  Report
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
	Launch  func(context.Context) (*exec.Cmd, error)
}

func (b *Bridge) Snapshot() Report {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.started {
		b.started = true
		ctx, cancel := context.WithCancel(context.Background())
		b.cancel = cancel
		b.done = make(chan struct{})
		b.report = Report{Problems: []string{"Hardware sensors starting…"}}
		go b.run(ctx)
	}
	r := b.report
	if !r.At.IsZero() && time.Since(r.At) > 15*time.Second {
		return Report{Problems: []string{"Hardware sensor readings expired; helper is restarting."}}
	}
	return r
}
func (b *Bridge) Close() {
	b.mu.Lock()
	cancel, done := b.cancel, b.done
	b.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
func (b *Bridge) publish(r Report) { b.mu.Lock(); b.report = r; b.mu.Unlock() }
func (b *Bridge) run(ctx context.Context) {
	defer close(b.done)
	for ctx.Err() == nil {
		if err := b.session(ctx); err != nil && ctx.Err() == nil {
			b.publish(Report{Problems: []string{"Hardware sensor helper: " + err.Error()}})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}
func (b *Bridge) session(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd, err := b.Launch(childCtx)
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()
	if err = cmd.Start(); err != nil {
		return err
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	type response struct {
		report Report
		err    error
	}
	for {
		// Include pipe writes and cold library initialization in the deadline.
		ch := make(chan response, 1)
		go func() {
			if _, err := io.WriteString(stdin, "sample\n"); err != nil {
				ch <- response{err: err}
				return
			}
			if !scanner.Scan() {
				err := scanner.Err()
				if err == nil {
					err = io.EOF
				}
				ch <- response{err: err}
				return
			}
			r, err := Decode(scanner.Bytes())
			ch <- response{r, err}
		}()
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("sample timed out")
		case result := <-ch:
			timer.Stop()
			if result.err != nil {
				return result.err
			}
			b.publish(result.report)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}
func Decode(data []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("invalid report: %w", err)
	}
	if r.Protocol != 1 {
		return Report{}, fmt.Errorf("unsupported protocol %d", r.Protocol)
	}
	if r.At.IsZero() || time.Since(r.At) > 15*time.Second || time.Until(r.At) > time.Minute {
		return Report{}, fmt.Errorf("invalid sample timestamp")
	}
	return r, nil
}
