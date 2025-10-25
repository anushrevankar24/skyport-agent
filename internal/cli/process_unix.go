//go:build unix

package cli

import (
	"os/exec"
	"syscall"
)

// configureDaemonProcess configures the command to run as a daemon process
// on Unix-like systems (Linux, macOS, etc.)
func configureDaemonProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create a new session and detach from terminal
	}
}
