//go:build ignore

// Emit labeled OS fixtures through the actual renderer for the artwork gallery.
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/midona-rhel/mosaic/internal/metrics"
	"github.com/midona-rhel/mosaic/internal/ui"
)

func main() {
	type frame struct {
		Platform      string
		Width, Height int
		Content       string
	}
	frames := []frame{}
	for _, size := range [][2]int{{120, 30}, {80, 24}} {
		for _, platform := range []string{"macOS", "Windows", "arch", "ubuntu", "fedora", "linuxmint", "debian", "alpine", "nixos", "gentoo", "manjaro", "opensuse", "rhel", "pop"} {
			m := ui.New(context.Background(), ui.Options{Demo: true})
			s := metrics.Demo()
			s.Host.OS, s.Host.Platform, s.Host.PlatformVersion = "linux", platform, ""
			s.Host.KernelVersion, s.Host.KernelArch = "6.17.0", "x86_64"
			switch platform {
			case "macOS":
				s.Host.OS, s.Host.PlatformVersion, s.Host.KernelVersion, s.Host.KernelArch = "darwin", "26.0", "25.0.0", "arm64"
			case "Windows":
				s.Host.OS, s.Host.PlatformVersion, s.Host.KernelVersion = "windows", "11", "10.0.26100"
			}
			m.SetSize(size[0], size[1])
			m.SetSnapshot(s)
			if len(os.Args) > 1 && os.Args[1] == "--cards" {
				workspace := m.Workspace()
				workspace.Spans[0] = ui.Span{W: 2, H: 1}
				if err := m.ApplyWorkspace(workspace); err != nil {
					panic(err)
				}
			} else if err := m.OpenPanel("system"); err != nil {
				panic(err)
			}
			frames = append(frames, frame{platform, size[0], size[1], m.View().Content})
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(frames); err != nil {
		panic(err)
	}
}
