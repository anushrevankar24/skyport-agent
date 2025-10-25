//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

// configureDaemonProcess configures the command to run as a daemon process
// on Windows systems
func configureDaemonProcess(cmd *exec.Cmd) {
	// On Windows, we use CREATE_NEW_PROCESS_GROUP to detach from the parent
	// 0x00000200 = CREATE_NEW_PROCESS_GROUP
	// 0x00000008 = DETACHED_PROCESS
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200 | 0x00000008,
	}
}
