//go:build !windows

package sshclient

import (
	"errors"
	"net"
	"os"
	"time"
)

func dialAgent() (net.Conn, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, errors.New("SSH_AUTH_SOCK is not set")
	}
	return net.DialTimeout("unix", socket, time.Second)
}
