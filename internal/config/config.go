package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Server struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	User             string `json:"user"`
	Shell            string `json:"shell"`
	Identity         string `json:"identity,omitempty"`
	Fingerprint      string `json:"fingerprint,omitempty"`
	Favorite         bool   `json:"favorite,omitempty"`
	PasswordSaved    bool   `json:"passwordSaved,omitempty"`
	RequireBiometric bool   `json:"requireBiometric,omitempty"`
}

type Preferences struct {
	DefaultUser     string `json:"defaultUser"`
	DefaultPort     int    `json:"defaultPort"`
	DefaultShell    string `json:"defaultShell"`
	DefaultIdentity string `json:"defaultIdentity,omitempty"`
	LogActivity     bool   `json:"logActivity"`
	Scrollback      int    `json:"scrollback"`
}

type Config struct {
	Servers     []Server    `json:"servers"`
	Preferences Preferences `json:"preferences"`
}

type Store struct {
	dir  string
	path string
}

func NewStore() (*Store, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "sshking")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, path: filepath.Join(dir, "config.json")}, nil
}

func Default() Config {
	user := os.Getenv("USER")
	if runtime.GOOS == "windows" {
		user = os.Getenv("USERNAME")
	}
	if user == "" {
		user = "admin"
	}
	return Config{
		Servers: []Server{},
		Preferences: Preferences{
			DefaultUser:  user,
			DefaultPort:  22,
			DefaultShell: "default",
			LogActivity:  true,
			Scrollback:   2000,
		},
	}
}

func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		return cfg, s.Save(cfg)
	}
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	normalize(&cfg)
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	normalize(&cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Log(serverName, kind, message string) error {
	logDir := filepath.Join(s.dir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, serverName)
	if safeName == "" {
		safeName = "session"
	}
	path := filepath.Join(logDir, time.Now().Format("2006-01-02")+"-"+safeName+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(time.Now().Format(time.RFC3339) + " [" + kind + "] " + message + "\n")
	return err
}

func normalize(cfg *Config) {
	if cfg.Preferences.DefaultPort <= 0 {
		cfg.Preferences.DefaultPort = 22
	}
	if cfg.Preferences.DefaultShell == "" {
		cfg.Preferences.DefaultShell = "default"
	}
	if cfg.Preferences.Scrollback < 100 {
		cfg.Preferences.Scrollback = 100
	}
	if cfg.Preferences.Scrollback > 10000 {
		cfg.Preferences.Scrollback = 10000
	}
	for i := range cfg.Servers {
		if cfg.Servers[i].Port <= 0 {
			cfg.Servers[i].Port = 22
		}
		if cfg.Servers[i].Shell == "" {
			cfg.Servers[i].Shell = cfg.Preferences.DefaultShell
		}
	}
}
