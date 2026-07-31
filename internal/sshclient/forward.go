package sshclient

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type Forward struct {
	listener net.Listener
	once     sync.Once
}

func (s *Session) ForwardLocal(localAddress, remoteAddress string) (*Forward, error) {
	listener, err := net.Listen("tcp", localAddress)
	if err != nil {
		return nil, err
	}
	forward := &Forward{listener: listener}
	go forward.serve(s, remoteAddress)
	return forward, nil
}

func (f *Forward) Address() string { return f.listener.Addr().String() }

func (f *Forward) Close() error {
	var err error
	f.once.Do(func() { err = f.listener.Close() })
	return err
}

func (f *Forward) serve(session *Session, remoteAddress string) {
	for {
		local, err := f.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer local.Close()
			remote, dialErr := session.client.Dial("tcp", remoteAddress)
			if dialErr != nil {
				return
			}
			defer remote.Close()
			var wait sync.WaitGroup
			wait.Add(2)
			go func() { defer wait.Done(); _, _ = io.Copy(remote, local) }()
			go func() { defer wait.Done(); _, _ = io.Copy(local, remote) }()
			wait.Wait()
		}()
	}
}

func TunnelAddress(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}
