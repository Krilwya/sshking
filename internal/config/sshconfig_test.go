package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportOpenSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	data := "Host *.internal\n  User shared\n\nHost prod production\n  HostName 10.0.0.8\n  User deploy\n  Port 2222\n  IdentityFile ~/.ssh/prod\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, err := ImportOpenSSH(path, Preferences{DefaultUser: "admin", DefaultPort: 22, DefaultShell: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Name != "prod" || servers[0].Host != "10.0.0.8" || servers[0].User != "deploy" || servers[0].Port != 2222 || servers[0].Identity != "~/.ssh/prod" {
		t.Fatalf("unexpected first server: %#v", servers[0])
	}
}

func TestActivityReturnsNewestFirstAndLimits(t *testing.T) {
	store := &Store{dir: t.TempDir()}
	if err := store.Log("alpha", "connect", "first"); err != nil {
		t.Fatal(err)
	}
	if err := store.Log("alpha", "command", "second"); err != nil {
		t.Fatal(err)
	}
	lines, err := store.Activity(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "second") {
		t.Fatalf("unexpected activity: %#v", lines)
	}
}
