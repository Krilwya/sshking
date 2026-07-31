package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyConfigMigratesServersToPersonal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"servers":[{"id":"legacy","name":"Legacy","host":"legacy.example.com","port":22,"user":"admin","shell":"default"}],"preferences":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{dir: dir, path: path}
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams == nil || len(cfg.Teams) != 0 {
		t.Fatalf("legacy config should start with an empty team collection: %#v", cfg.Teams)
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0].TeamID != "" {
		t.Fatalf("legacy server should migrate to Personal Servers: %#v", cfg.Servers)
	}
}

func TestNormalizePreservesValidTeamOwnershipAndRepairsUnknownTeam(t *testing.T) {
	cfg := Default()
	cfg.Teams = []Team{{ID: "ops", Name: " Operations "}}
	cfg.Servers = []Server{
		{ID: "valid", TeamID: "ops", Name: "Valid", Port: 22, Shell: "default"},
		{ID: "orphan", TeamID: "missing", Name: "Orphan", Port: 22, Shell: "default"},
	}
	normalize(&cfg)
	if cfg.Teams[0].Name != "Operations" || cfg.Servers[0].TeamID != "ops" {
		t.Fatalf("valid team ownership was not preserved: %#v", cfg)
	}
	if cfg.Servers[1].TeamID != "" {
		t.Fatalf("unknown team should migrate back to Personal Servers: %#v", cfg.Servers[1])
	}
}
