package metrics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Only the child test process created below is eligible for this live action.
func TestTerminationHelper(t *testing.T) {
	if os.Getenv("MOSAIC_TERMINATION_HELPER") != "1" {
		return
	}
	fmt.Println("ready")
	time.Sleep(30 * time.Second)
	os.Exit(0)
}
func TestTerminateOwnedChild(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestTerminationHelper$")
	cmd.Env = append(os.Environ(), "MOSAIC_TERMINATION_HELPER=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(out).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected child greeting %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child failed to start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p, err := process.NewProcessWithContext(ctx, int32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	started, err := p.CreateTimeWithContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target := Process{PID: p.Pid, Started: started}
	if _, err = Inspect(ctx, target); err != nil {
		t.Fatal(err)
	}
	wrong := target
	wrong.Started++
	if Terminate(ctx, wrong) == nil {
		t.Fatal("wrong process identity accepted")
	}
	if err = Terminate(ctx, target); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("child exited normally instead of being terminated")
		}
	case <-ctx.Done():
		t.Fatal("termination did not stop owned child")
	}
}
