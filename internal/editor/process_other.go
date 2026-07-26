//go:build !windows

package editor

import "os/exec"

func hideCommandWindow(*exec.Cmd) {}
