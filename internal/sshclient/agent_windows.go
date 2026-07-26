//go:build windows

package sshclient

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

func dialAgent() (net.Conn, error) {
	return winio.DialPipe(`\\.\pipe\openssh-ssh-agent`, ptrDuration(time.Second))
}

func ptrDuration(value time.Duration) *time.Duration {
	return &value
}
