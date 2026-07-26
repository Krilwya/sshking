package sshclient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"sshking/internal/config"
)

type Session struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	output  chan string
	done    chan error
	once    sync.Once
	writeMu sync.Mutex
}

func Connect(server config.Server, password string) (*Session, error) {
	client, err := dial(server, password)
	if err != nil {
		return nil, err
	}
	ss, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	if err := ss.RequestPty("xterm-256color", 34, 120, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		ss.Close()
		client.Close()
		return nil, err
	}
	stdin, err := ss.StdinPipe()
	if err != nil {
		ss.Close()
		client.Close()
		return nil, err
	}
	stdout, err := ss.StdoutPipe()
	if err != nil {
		ss.Close()
		client.Close()
		return nil, err
	}
	stderr, err := ss.StderrPipe()
	if err != nil {
		ss.Close()
		client.Close()
		return nil, err
	}
	s := &Session{
		client: client, session: ss, stdin: stdin,
		output: make(chan string, 64), done: make(chan error, 1),
	}
	command := shellCommand(server.Shell)
	if command == "" {
		err = ss.Shell()
	} else {
		err = ss.Start(command)
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	go s.read(stdout)
	go s.read(stderr)
	go func() {
		s.done <- ss.Wait()
		close(s.done)
	}()
	return s, nil
}

func dial(server config.Server, password string) (*ssh.Client, error) {
	auth, err := authMethods(server.Identity, password)
	if err != nil {
		return nil, err
	}
	if len(auth) == 0 {
		return nil, errors.New("no SSH authentication method available; configure a private key, password, or SSH agent")
	}
	hostKey := ssh.InsecureIgnoreHostKey()
	if expected := strings.TrimSpace(server.Fingerprint); expected != "" {
		hostKey = func(_ string, _ net.Addr, key ssh.PublicKey) error {
			actual := ssh.FingerprintSHA256(key)
			if actual != expected {
				return fmt.Errorf("host key mismatch: got %s", actual)
			}
			return nil
		}
	}
	sshCfg := &ssh.ClientConfig{
		User:            server.User,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         10 * time.Second,
	}
	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	return ssh.Dial("tcp", address, sshCfg)
}

func (s *Session) Output() <-chan string { return s.output }
func (s *Session) Done() <-chan error    { return s.done }

func (s *Session) Send(line string) error {
	return s.SendInput(line + "\r")
}

// SendInput writes unmodified terminal input to the remote PTY. This includes
// control characters and VT key sequences such as Ctrl+C and arrow keys.
func (s *Session) SendInput(data string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := io.WriteString(s.stdin, data)
	return err
}

func (s *Session) Resize(columns, rows int) error {
	if columns < 20 || rows < 4 {
		return nil
	}
	return s.session.WindowChange(rows, columns)
}

func (s *Session) Close() error {
	var err error
	s.once.Do(func() {
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
		if s.session != nil {
			err = s.session.Close()
		}
		if s.client != nil {
			_ = s.client.Close()
		}
	})
	return err
}

func (s *Session) read(r io.Reader) {
	reader := bufio.NewReaderSize(r, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			s.output <- string(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func shellCommand(shell string) string {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "", "default":
		return ""
	case "zsh", "bash", "fish":
		return "exec " + strings.ToLower(shell) + " -l"
	default:
		return "exec " + shell
	}
}

func authMethods(identity, password string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if password != "" {
		methods = append(methods, ssh.Password(password))
	}
	if identity != "" {
		path := expandHome(identity)
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read identity: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse identity: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if conn, err := dialAgent(); err == nil {
		methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
	}
	return methods, nil
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimLeft(path[1:], `/\`))
		}
	}
	return path
}
