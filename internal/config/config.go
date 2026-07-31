package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Server struct {
	ID               string `json:"id"`
	TeamID           string `json:"teamId,omitempty"`
	Name             string `json:"name"`
	Group            string `json:"group,omitempty"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	User             string `json:"user"`
	Shell            string `json:"shell"`
	UseTmux          bool   `json:"useTmux,omitempty"`
	TmuxSession      string `json:"tmuxSession,omitempty"`
	Identity         string `json:"identity,omitempty"`
	JumpServerID     string `json:"jumpServerId,omitempty"`
	Fingerprint      string `json:"fingerprint,omitempty"`
	Favorite         bool   `json:"favorite,omitempty"`
	PasswordSaved    bool   `json:"passwordSaved,omitempty"`
	RequireBiometric bool   `json:"requireBiometric,omitempty"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Preferences struct {
	CloudURL               string `json:"cloudUrl,omitempty"`
	DefaultUser            string `json:"defaultUser"`
	DefaultPort            int    `json:"defaultPort"`
	DefaultShell           string `json:"defaultShell"`
	DefaultIdentity        string `json:"defaultIdentity,omitempty"`
	LogActivity            bool   `json:"logActivity"`
	Scrollback             int    `json:"scrollback"`
	Theme                  string `json:"theme"`
	UIScale                int    `json:"uiScale"`
	TerminalFontSize       int    `json:"terminalFontSize"`
	TerminalFontFamily     string `json:"terminalFontFamily"`
	TerminalLineHeight     int    `json:"terminalLineHeight"`
	AutoConnectTabs        bool   `json:"autoConnectTabs"`
	ReopenActiveSession    bool   `json:"reopenActiveSession"`
	PersistTerminalHistory bool   `json:"persistTerminalHistory"`
}

type Config struct {
	Servers     []Server    `json:"servers"`
	Teams       []Team      `json:"teams"`
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
		Teams:   []Team{},
		Preferences: Preferences{
			CloudURL:               "https://cloud.krilwya.fr",
			DefaultUser:            user,
			DefaultPort:            22,
			DefaultShell:           "default",
			LogActivity:            true,
			Scrollback:             2000,
			Theme:                  "glass",
			UIScale:                100,
			TerminalFontSize:       14,
			TerminalFontFamily:     "system-mono",
			TerminalLineHeight:     140,
			AutoConnectTabs:        true,
			ReopenActiveSession:    true,
			PersistTerminalHistory: true,
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

func (s *Store) Activity(limit int) ([]string, error) {
	if limit < 1 {
		limit = 200
	}
	logDir := filepath.Join(s.dir, "logs")
	files, err := os.ReadDir(logDir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	var lines []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".log") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(logDir, file.Name()))
		if readErr != nil {
			return nil, readErr
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, file.Name()+"  "+line)
			}
		}
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return lines, nil
}

func normalize(cfg *Config) {
	cfg.Preferences.CloudURL = strings.TrimRight(strings.TrimSpace(cfg.Preferences.CloudURL), "/")
	if cfg.Servers == nil {
		cfg.Servers = []Server{}
	}
	if cfg.Teams == nil {
		cfg.Teams = []Team{}
	}
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
	switch cfg.Preferences.Theme {
	case "light", "black", "glass":
	default:
		cfg.Preferences.Theme = "glass"
	}
	if cfg.Preferences.UIScale < 80 || cfg.Preferences.UIScale > 140 {
		cfg.Preferences.UIScale = 100
	}
	if cfg.Preferences.TerminalFontSize < 10 || cfg.Preferences.TerminalFontSize > 28 {
		cfg.Preferences.TerminalFontSize = 14
	}
	switch cfg.Preferences.TerminalFontFamily {
	case "system-mono", "cascadia", "jetbrains", "source-code":
	default:
		cfg.Preferences.TerminalFontFamily = "system-mono"
	}
	if cfg.Preferences.TerminalLineHeight < 100 || cfg.Preferences.TerminalLineHeight > 200 {
		cfg.Preferences.TerminalLineHeight = 140
	}
	teamIDs := make(map[string]struct{}, len(cfg.Teams))
	for i := range cfg.Teams {
		cfg.Teams[i].Name = strings.TrimSpace(cfg.Teams[i].Name)
		teamIDs[cfg.Teams[i].ID] = struct{}{}
	}
	for i := range cfg.Servers {
		if cfg.Servers[i].TeamID != "" {
			if _, exists := teamIDs[cfg.Servers[i].TeamID]; !exists {
				cfg.Servers[i].TeamID = ""
			}
		}
		if cfg.Servers[i].Port <= 0 {
			cfg.Servers[i].Port = 22
		}
		if cfg.Servers[i].Shell == "" {
			cfg.Servers[i].Shell = cfg.Preferences.DefaultShell
		}
		if cfg.Servers[i].UseTmux && strings.TrimSpace(cfg.Servers[i].TmuxSession) == "" {
			cfg.Servers[i].TmuxSession = "sshking"
		}
	}
}
