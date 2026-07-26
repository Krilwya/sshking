//go:build windows

package editor

import (
	"os/exec"
	"syscall"
)

func hideCommandWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
