package sshclient

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"

	"sshking/internal/config"
)

type PublicKeyInfo struct {
	Path        string `json:"path"`
	PrivatePath string `json:"privatePath,omitempty"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}

func ListPublicKeys() ([]PublicKeyInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	matches, err := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	if err != nil {
		return nil, fmt.Errorf("list SSH keys: %w", err)
	}
	keys := make([]PublicKeyInfo, 0, len(matches))
	for _, path := range matches {
		info, err := publicKeyInfo(path)
		if err == nil {
			keys = append(keys, info)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i].Name) < strings.ToLower(keys[j].Name)
	})
	return keys, nil
}

func GenerateServerKey(server config.Server) (PublicKeyInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("find home directory: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return PublicKeyInfo{}, fmt.Errorf("create SSH directory: %w", err)
	}
	base := filepath.Join(sshDir, "sshking_"+keySlug(server.Name))
	publicPath := base + ".pub"
	if _, privateErr := os.Stat(base); privateErr == nil {
		if _, publicErr := os.Stat(publicPath); publicErr == nil {
			return publicKeyInfo(publicPath)
		}
		return PublicKeyInfo{}, fmt.Errorf("private key already exists at %s but its public key is missing", base)
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("generate Ed25519 key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "SSHKing "+server.Name)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("encode private key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("encode public key: %w", err)
	}
	privateData := pem.EncodeToMemory(block)
	publicData := ssh.MarshalAuthorizedKey(signer.PublicKey())
	publicData = append([]byte(strings.TrimSpace(string(publicData))+" SSHKing "+server.Name), '\n')

	privateFile, err := os.OpenFile(base, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("create private key: %w", err)
	}
	if _, err := privateFile.Write(privateData); err != nil {
		privateFile.Close()
		_ = os.Remove(base)
		return PublicKeyInfo{}, fmt.Errorf("write private key: %w", err)
	}
	if err := privateFile.Close(); err != nil {
		_ = os.Remove(base)
		return PublicKeyInfo{}, fmt.Errorf("close private key: %w", err)
	}
	if err := os.WriteFile(publicPath, publicData, 0o644); err != nil {
		_ = os.Remove(base)
		return PublicKeyInfo{}, fmt.Errorf("write public key: %w", err)
	}
	return publicKeyInfo(publicPath)
}

func InstallPublicKey(server config.Server, publicKeyPath, password string) (PublicKeyInfo, error) {
	info, err := publicKeyInfo(expandHome(strings.TrimSpace(publicKeyPath)))
	if err != nil {
		return PublicKeyInfo{}, err
	}
	publicData, err := os.ReadFile(info.Path)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("read public key: %w", err)
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(publicData)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("parse public key: %w", err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))

	client, err := dial(server, password)
	if err != nil {
		return PublicKeyInfo{}, err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("open key installation session: %w", err)
	}
	defer session.Close()
	session.Stdin = strings.NewReader(line + "\n")
	const command = `umask 077; mkdir -p "$HOME/.ssh" && touch "$HOME/.ssh/authorized_keys" && chmod 700 "$HOME/.ssh" && chmod 600 "$HOME/.ssh/authorized_keys" && key=$(cat) && { grep -qxF "$key" "$HOME/.ssh/authorized_keys" || printf '%s\n' "$key" >> "$HOME/.ssh/authorized_keys"; }`
	if output, err := session.CombinedOutput(command); err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return PublicKeyInfo{}, fmt.Errorf("install public key: %w: %s", err, message)
		}
		return PublicKeyInfo{}, fmt.Errorf("install public key: %w", err)
	}
	return info, nil
}

func publicKeyInfo(path string) (PublicKeyInfo, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("resolve public key path: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pub") {
		return PublicKeyInfo{}, errors.New("select a .pub public key file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("read public key: %w", err)
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return PublicKeyInfo{}, fmt.Errorf("parse public key: %w", err)
	}
	privatePath := strings.TrimSuffix(path, filepath.Ext(path))
	if _, err := os.Stat(privatePath); err != nil {
		privatePath = ""
	}
	return PublicKeyInfo{
		Path:        path,
		PrivatePath: privatePath,
		Name:        filepath.Base(path),
		Fingerprint: ssh.FingerprintSHA256(key),
	}, nil
}

func keySlug(name string) string {
	var result strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			result.WriteRune(r)
		case result.Len() > 0 && !strings.HasSuffix(result.String(), "_"):
			result.WriteByte('_')
		}
	}
	slug := strings.Trim(result.String(), "_")
	if slug == "" {
		return "server"
	}
	return slug
}
