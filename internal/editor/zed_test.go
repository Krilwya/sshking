package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sshking/internal/config"
)

func TestZedSSHURL(t *testing.T) {
	server := config.Server{Host: "example.com", Port: 2222, User: "deploy"}
	tests := map[string]string{
		"":                "ssh://deploy@example.com:2222/~",
		"~":               "ssh://deploy@example.com:2222/~",
		"~/project":       "ssh://deploy@example.com:2222/~/project",
		"project/file.go": "ssh://deploy@example.com:2222/~/project/file.go",
		"/srv/my project": "ssh://deploy@example.com:2222/srv/my%20project",
	}
	for input, want := range tests {
		if got := zedSSHURL(server, input, ""); got != want {
			t.Errorf("zedSSHURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestZedSSHURLUsesAliasWithoutPort(t *testing.T) {
	server := config.Server{Host: "sshking-abc123", Port: 0, User: "deploy"}
	got := zedSSHURL(server, "~/project", "")
	want := "ssh://deploy@sshking-abc123/~/project"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestZedSSHURLEncodesPassword(t *testing.T) {
	server := config.Server{Host: "example.com", Port: 22, User: "deploy"}
	got := zedSSHURL(server, "~/project", "p@ss:/ word")
	want := "ssh://deploy:p%40ss%3A%2F%20word@example.com:22/~/project"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEnsureZedSSHConfigAt(t *testing.T) {
	root := t.TempDir()
	sshDir := filepath.Join(root, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	identity := filepath.Join(sshDir, "work key")
	if err := os.WriteFile(identity, []byte("test key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte("Host existing\n  HostName old.example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := config.Server{
		ID:       "server-1",
		Host:     "example.com",
		Port:     2222,
		User:     "deploy",
		Identity: identity,
	}

	alias, err := ensureZedSSHConfigAt(sshDir, server)
	if err != nil {
		t.Fatal(err)
	}
	mainConfig, err := os.ReadFile(filepath.Join(sshDir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(mainConfig), sshKingInclude+"\n\n") {
		t.Fatalf("main config does not start with SSHKing include:\n%s", mainConfig)
	}
	if !strings.Contains(string(mainConfig), "Host existing") {
		t.Fatalf("existing config was not preserved:\n%s", mainConfig)
	}

	managed, err := os.ReadFile(filepath.Join(sshDir, "sshking_config"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Host " + alias,
		"HostName example.com",
		"User deploy",
		"Port 2222",
		`IdentityFile "` + filepath.ToSlash(identity) + `"`,
		"IdentitiesOnly yes",
	} {
		if !strings.Contains(string(managed), want) {
			t.Errorf("managed config missing %q:\n%s", want, managed)
		}
	}

	if _, err := ensureZedSSHConfigAt(sshDir, server); err != nil {
		t.Fatal(err)
	}
	managed, err = os.ReadFile(filepath.Join(sshDir, "sshking_config"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(managed), "Host "+alias) != 1 {
		t.Fatalf("host block was duplicated:\n%s", managed)
	}
}
