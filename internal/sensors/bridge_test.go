package sensors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBridgeChild(t *testing.T) {
	mode := os.Getenv("MOSAIC_SENSOR_TEST_CHILD")
	if mode == "" {
		return
	}
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		switch mode {
		case "bad":
			fmt.Println(`{"Protocol":9}`)
		case "hang":
			time.Sleep(time.Minute)
		default:
			b, _ := json.Marshal(Report{Protocol: 1, At: time.Now(), Sensors: []Sensor{{ID: "cpu", Type: "Temperature"}}})
			fmt.Println(string(b))
		}
	}
	if marker := os.Getenv("MOSAIC_SENSOR_EXIT_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("closed"), 0600)
	}
	os.Exit(0)
}
func testBridge(mode string) *Bridge {
	return &Bridge{Launch: func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBridgeChild$")
		cmd.Env = append(os.Environ(), "MOSAIC_SENSOR_TEST_CHILD="+mode)
		return cmd, nil
	}}
}
func TestBridgeReportAndShutdown(t *testing.T) {
	b := testBridge("good")
	defer b.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r := b.Snapshot()
		if r.Protocol == 1 {
			if len(r.Sensors) != 1 || r.Sensors[0].Value != nil {
				t.Fatal("missing value became a measurement")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("helper failed to deliver a report")
}
func TestBridgeMalformedAndHungHelpers(t *testing.T) {
	for _, mode := range []string{"bad", "hang"} {
		t.Run(mode, func(t *testing.T) {
			b := testBridge(mode)
			b.Snapshot()
			if mode == "bad" {
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) && strings.Contains(strings.Join(b.Snapshot().Problems, " "), "starting") {
					time.Sleep(10 * time.Millisecond)
				}
				if !strings.Contains(strings.Join(b.Snapshot().Problems, " "), "unsupported protocol") {
					t.Fatal(b.Snapshot())
				}
			}
			done := make(chan struct{})
			go func() { b.Close(); close(done) }()
			select {
			case <-done:
			case <-time.After(7 * time.Second):
				t.Fatal("helper did not terminate")
			}
		})
	}
}
func TestDecodeRejectsStaleReports(t *testing.T) {
	for _, at := range []time.Time{{}, time.Now().Add(-time.Minute), time.Now().Add(time.Hour)} {
		data, _ := json.Marshal(Report{Protocol: 1, At: at})
		if _, err := Decode(data); err == nil {
			t.Fatal("accepted invalid sample age")
		}
	}
}

func TestBridgeClosesInputBeforeKilling(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "closed")
	b := testBridge("good")
	launch := b.Launch
	b.Launch = func(ctx context.Context) (*exec.Cmd, error) {
		cmd, err := launch(ctx)
		if err == nil {
			cmd.Env = append(cmd.Env, "MOSAIC_SENSOR_EXIT_MARKER="+marker)
		}
		return cmd, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for b.Snapshot().Protocol != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	b.Close()
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("helper did not get a chance to clean up before exit:", err)
	}
}
