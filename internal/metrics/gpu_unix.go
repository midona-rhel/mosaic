//go:build !windows

package metrics

import "os/exec"

func configureGPUCommand(cmd *exec.Cmd) {}
