package editor

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"sshking/internal/config"
)

func OpenZed(server config.Server, remotePath string, newWindow bool, password string) error {
	executable, err := findZed()
	if err != nil {
		return err
	}

	args := make([]string, 0, 2)
	if newWindow {
		args = append(args, "--new")
	} else {
		args = append(args, "--existing")
	}
	args = append(args, zedSSHURL(server, remotePath, password))

	command := exec.Command(executable, args...)
	hideCommandWindow(command)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Zed: %w", err)
	}
	return nil
}

func zedSSHURL(server config.Server, remotePath, password string) string {
	path := strings.TrimSpace(remotePath)
	switch {
	case path == "", path == "~":
		path = "/~"
	case strings.HasPrefix(path, "~/"):
		path = "/" + path
	case !strings.HasPrefix(path, "/"):
		path = "/~/" + path
	}

	user := url.User(server.User)
	if password != "" {
		user = url.UserPassword(server.User, password)
	}
	return (&url.URL{
		Scheme: "ssh",
		User:   user,
		Host:   net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port)),
		Path:   path,
	}).String()
}

func findZed() (string, error) {
	for _, name := range []string{"zed", "zed.exe", "Zed.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	var candidates []string
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Programs", "Zed", "bin", "Zed.exe"),
				filepath.Join(localAppData, "Programs", "Zed", "Zed.exe"),
			)
		}
	} else if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/usr/local/bin/zed",
			"/opt/homebrew/bin/zed",
			"/Applications/Zed.app/Contents/MacOS/cli",
		)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("Zed CLI was not found; install it from Zed’s command palette with “cli: install cli binary”")
}
