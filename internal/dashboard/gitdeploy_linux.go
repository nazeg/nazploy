//go:build linux
// +build linux

package dashboard

import (
	"os/exec"
	"syscall"
)

func setProcAttributes(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Negative PID kills the process group
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
