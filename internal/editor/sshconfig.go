package editor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sshking/internal/config"
)

const sshKingInclude = "Include ~/.ssh/sshking_config"

func ensureZedSSHConfig(server config.Server) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return ensureZedSSHConfigAt(filepath.Join(home, ".ssh"), server)
}

func ensureZedSSHConfigAt(sshDir string, server config.Server) (string, error) {
	identity := expandIdentity(server.Identity)
	info, err := os.Stat(identity)
	if err != nil {
		return "", fmt.Errorf("configured private key is unavailable: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("configured private key is a directory: %s", identity)
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("create SSH directory: %w", err)
	}

	configPath := filepath.Join(sshDir, "config")
	configData, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read SSH config: %w", err)
	}
	configText := string(configData)
	if !hasSSHKingInclude(configText) {
		prefix := sshKingInclude + "\n\n"
		configText = prefix + strings.TrimLeft(configText, "\r\n")
		if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
			return "", fmt.Errorf("update SSH config: %w", err)
		}
	}

	alias := zedHostAlias(server)
	managedPath := filepath.Join(sshDir, "sshking_config")
	managedData, err := os.ReadFile(managedPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read SSHKing SSH config: %w", err)
	}
	managedText := upsertHostBlock(string(managedData), alias, server, identity)
	if err := os.WriteFile(managedPath, []byte(managedText), 0o600); err != nil {
		return "", fmt.Errorf("write SSHKing SSH config: %w", err)
	}
	return alias, nil
}

func hasSSHKingInclude(configText string) bool {
	for _, line := range strings.Split(configText, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.EqualFold(line, sshKingInclude) {
			return true
		}
	}
	return false
}

func upsertHostBlock(contents, alias string, server config.Server, identity string) string {
	begin := "# BEGIN SSHKing " + alias
	end := "# END SSHKing " + alias
	block := strings.Join([]string{
		begin,
		"Host " + alias,
		"  HostName " + sshConfigValue(server.Host),
		"  User " + sshConfigValue(server.User),
		fmt.Sprintf("  Port %d", server.Port),
		"  IdentityFile " + sshConfigValue(filepath.ToSlash(identity)),
		"  IdentitiesOnly yes",
		end,
	}, "\n")

	start := strings.Index(contents, begin)
	if start >= 0 {
		stop := strings.Index(contents[start:], end)
		if stop >= 0 {
			stop = start + stop + len(end)
			return strings.TrimSpace(contents[:start]+block+contents[stop:]) + "\n"
		}
	}
	if strings.TrimSpace(contents) == "" {
		return block + "\n"
	}
	return strings.TrimSpace(contents) + "\n\n" + block + "\n"
}

func zedHostAlias(server config.Server) string {
	seed := server.ID
	if strings.TrimSpace(seed) == "" {
		seed = server.User + "@" + server.Host + ":" + strconv.Itoa(server.Port)
	}
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("sshking-%x", sum[:6])
}

func sshConfigValue(value string) string {
	value = strings.TrimSpace(value)
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"'#") {
		return value
	}
	return strconv.Quote(value)
}

func expandIdentity(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimLeft(path[1:], `/\`))
		}
	}
	return path
}
