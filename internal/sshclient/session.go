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

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"sshking/internal/config"
)

type Session struct {
	client             *ssh.Client
	jumpClient         *ssh.Client
	session            *ssh.Session
	stdin              io.WriteCloser
	output             chan string
	done               chan error
	closed             chan struct{}
	hostKeyFingerprint string
	once               sync.Once
	writeMu            sync.Mutex
}

// HostKeyVerificationError indicates that a server presented an unknown host
// key and the user must explicitly trust it before the connection can proceed.
type HostKeyVerificationError struct {
	Fingerprint string
}

func (e *HostKeyVerificationError) Error() string {
	return fmt.Sprintf("host key verification required: %s", e.Fingerprint)
}

func Connect(server config.Server, password string, trustNewHost bool) (*Session, error) {
	client, fingerprint, err := dialWithHostKeyPolicy(server, password, trustNewHost)
	if err != nil {
		return nil, err
	}
	return startInteractiveSession(client, nil, server, fingerprint)
}

func ConnectVia(server config.Server, password string, jump config.Server, jumpPassword string, trustNewHost bool) (*Session, error) {
	jumpClient, _, err := dialWithHostKeyPolicy(jump, jumpPassword, false)
	if err != nil {
		return nil, fmt.Errorf("connect jump host %s: %w", jump.Name, err)
	}
	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	connection, err := jumpClient.Dial("tcp", address)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("jump to %s: %w", server.Name, err)
	}
	sshConfig, fingerprint, err := clientConfig(server, password, trustNewHost)
	if err != nil {
		connection.Close()
		jumpClient.Close()
		return nil, err
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, address, sshConfig)
	if err != nil {
		connection.Close()
		jumpClient.Close()
		return nil, err
	}
	client := ssh.NewClient(clientConnection, channels, requests)
	return startInteractiveSession(client, jumpClient, server, *fingerprint)
}

func startInteractiveSession(client, jumpClient *ssh.Client, server config.Server, fingerprint string) (*Session, error) {
	closeClients := func() {
		_ = client.Close()
		if jumpClient != nil {
			_ = jumpClient.Close()
		}
	}
	ss, err := client.NewSession()
	if err != nil {
		closeClients()
		return nil, err
	}
	if err := ss.RequestPty("xterm-256color", 34, 120, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		ss.Close()
		closeClients()
		return nil, err
	}
	stdin, err := ss.StdinPipe()
	if err != nil {
		ss.Close()
		closeClients()
		return nil, err
	}
	stdout, err := ss.StdoutPipe()
	if err != nil {
		ss.Close()
		closeClients()
		return nil, err
	}
	stderr, err := ss.StderrPipe()
	if err != nil {
		ss.Close()
		closeClients()
		return nil, err
	}
	s := &Session{
		client: client, jumpClient: jumpClient, session: ss, stdin: stdin,
		output: make(chan string, 64), done: make(chan error, 1),
		closed:             make(chan struct{}),
		hostKeyFingerprint: fingerprint,
	}
	command := interactiveCommand(server)
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
	go s.keepAlive()
	go func() {
		s.done <- ss.Wait()
		close(s.done)
	}()
	return s, nil
}

func dial(server config.Server, password string) (*ssh.Client, error) {
	client, _, err := dialWithHostKeyPolicy(server, password, false)
	return client, err
}

func dialWithHostKeyPolicy(server config.Server, password string, trustNewHost bool) (*ssh.Client, string, error) {
	sshCfg, fingerprint, err := clientConfig(server, password, trustNewHost)
	if err != nil {
		return nil, "", err
	}
	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	client, err := ssh.Dial("tcp", address, sshCfg)
	return client, *fingerprint, err
}

func clientConfig(server config.Server, password string, trustNewHost bool) (*ssh.ClientConfig, *string, error) {
	auth, err := authMethods(server.Identity, password)
	if err != nil {
		return nil, nil, err
	}
	if len(auth) == 0 {
		return nil, nil, errors.New("no SSH authentication method available; configure a private key, password, or SSH agent")
	}
	var actualFingerprint string
	var hostKey ssh.HostKeyCallback
	if expected := strings.TrimSpace(server.Fingerprint); expected != "" {
		hostKey = func(_ string, _ net.Addr, key ssh.PublicKey) error {
			actual := ssh.FingerprintSHA256(key)
			if actual != expected {
				return fmt.Errorf("host key mismatch: got %s", actual)
			}
			actualFingerprint = actual
			return nil
		}
	} else {
		knownHosts, knownHostsErr := knownHostsCallback()
		hostKey = func(host string, remote net.Addr, key ssh.PublicKey) error {
			actualFingerprint = ssh.FingerprintSHA256(key)
			if knownHosts != nil {
				if err := knownHosts(host, remote, key); err == nil {
					return nil
				} else {
					var keyErr *knownhosts.KeyError
					if !errors.As(err, &keyErr) {
						return err
					}
					if len(keyErr.Want) > 0 || !trustNewHost {
						if len(keyErr.Want) == 0 && !trustNewHost {
							return &HostKeyVerificationError{Fingerprint: actualFingerprint}
						}
						return err
					}
					return nil
				}
			}
			if knownHostsErr != nil && !errors.Is(knownHostsErr, os.ErrNotExist) {
				// A malformed known_hosts file should not be silently bypassed.
				return knownHostsErr
			}
			if !trustNewHost {
				return &HostKeyVerificationError{Fingerprint: actualFingerprint}
			}
			return nil
		}
	}
	return &ssh.ClientConfig{
		User:            server.User,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         10 * time.Second,
	}, &actualFingerprint, nil
}

func knownHostsCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory for known_hosts: %w", err)
	}
	return knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
}

func (s *Session) HostKeyFingerprint() string { return s.hostKeyFingerprint }

// AgentHasKeys reports whether the current device has at least one key loaded
// in its SSH agent. It deliberately does not expose key material.
func AgentHasKeys() bool {
	conn, err := dialAgent()
	if err != nil {
		return false
	}
	defer conn.Close()
	keys, err := agent.NewClient(conn).List()
	return err == nil && len(keys) > 0
}

func (s *Session) Output() <-chan string { return s.output }
func (s *Session) Done() <-chan error    { return s.done }

func (s *Session) NewSFTPClient() (*sftp.Client, error) {
	return sftp.NewClient(s.client)
}

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
		close(s.closed)
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
		if s.session != nil {
			err = s.session.Close()
		}
		if s.client != nil {
			_ = s.client.Close()
		}
		if s.jumpClient != nil {
			_ = s.jumpClient.Close()
		}
	})
	return err
}

func (s *Session) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ticker.C:
			_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
			if err == nil {
				failures = 0
				continue
			}
			failures++
			if failures >= 3 {
				_ = s.Close()
				return
			}
		case <-s.closed:
			return
		}
	}
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
	normalized := strings.ToLower(strings.TrimSpace(shell))
	switch normalized {
	case "", "default":
		return ""
	case "zsh", "bash", "fish":
		return "exec " + normalized + " -l"
	default:
		return "exec " + shell
	}
}

func interactiveCommand(server config.Server) string {
	if !server.UseTmux {
		return shellCommand(server.Shell)
	}
	name := strings.TrimSpace(server.TmuxSession)
	if name == "" {
		name = "sshking"
	}
	target := shellQuote(name)
	create := "tmux new-session -d -s " + target
	if selectedShell := shellCommand(server.Shell); selectedShell != "" {
		create += " " + shellQuote(selectedShell)
	}
	fallback := shellCommand(server.Shell)
	if fallback == "" {
		fallback = `exec "${SHELL:-/bin/sh}" -l`
	}
	return "if command -v tmux >/dev/null 2>&1; then " +
		"if ! tmux has-session -t " + target + " 2>/dev/null; then " + create + "; fi; " +
		"tmux set-option -t " + target + " mouse on >/dev/null 2>&1 || true; " +
		"exec tmux attach-session -t " + target + "; " +
		`else printf '\033[33mSSHKing: tmux is not installed; using a normal shell.\033[0m\r\n' >&2; ` + fallback + "; fi"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
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
