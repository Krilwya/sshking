package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sshking/internal/biometric"
	"sshking/internal/config"
	"sshking/internal/credentials"
	"sshking/internal/editor"
	"sshking/internal/sshclient"
)

type InitialState struct {
	Config             config.Config `json:"config"`
	Platform           string        `json:"platform"`
	ConfigDir          string        `json:"configDir"`
	BiometricAvailable bool          `json:"biometricAvailable"`
	BiometricName      string        `json:"biometricName"`
}

type App struct {
	ctx                context.Context
	store              *config.Store
	cfg                config.Config
	session            *sshclient.Session
	active             string
	mu                 sync.Mutex
	biometricAvailable bool
}

func NewApp() (*App, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &App{store: store, cfg: cfg, biometricAvailable: biometric.Available()}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.session != nil {
		_ = a.session.Close()
	}
}

func (a *App) GetState() InitialState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return InitialState{
		Config:             a.cfg,
		Platform:           runtime.GOOS,
		BiometricAvailable: a.biometricAvailable,
		BiometricName:      biometric.Name,
	}
}

func (a *App) SaveServer(server config.Server) (config.Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if server.Name == "" || server.Host == "" || server.User == "" {
		return a.cfg, errors.New("name, host, and user are required")
	}
	if server.Port < 1 || server.Port > 65535 {
		return a.cfg, errors.New("port must be between 1 and 65535")
	}
	if server.ID == "" {
		server.ID = newID()
		a.cfg.Servers = append(a.cfg.Servers, server)
	} else {
		found := false
		for i := range a.cfg.Servers {
			if a.cfg.Servers[i].ID == server.ID {
				a.cfg.Servers[i] = server
				found = true
				break
			}
		}
		if !found {
			a.cfg.Servers = append(a.cfg.Servers, server)
		}
	}
	if err := a.store.Save(a.cfg); err != nil {
		return a.cfg, err
	}
	return a.cfg, nil
}

func (a *App) DeleteServer(id string) (config.Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.cfg.Servers {
		if a.cfg.Servers[i].ID == id {
			if err := credentials.Delete(id); err != nil {
				return a.cfg, fmt.Errorf("delete saved password: %w", err)
			}
			a.cfg.Servers = append(a.cfg.Servers[:i], a.cfg.Servers[i+1:]...)
			if a.active == id && a.session != nil {
				_ = a.session.Close()
				a.session = nil
				a.active = ""
			}
			return a.cfg, a.store.Save(a.cfg)
		}
	}
	return a.cfg, errors.New("server not found")
}

func (a *App) SavePreferences(preferences config.Preferences) (config.Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if preferences.DefaultPort < 1 || preferences.DefaultPort > 65535 {
		return a.cfg, errors.New("default port must be between 1 and 65535")
	}
	if preferences.Scrollback < 100 || preferences.Scrollback > 10000 {
		return a.cfg, errors.New("scrollback must be between 100 and 10000")
	}
	a.cfg.Preferences = preferences
	return a.cfg, a.store.Save(a.cfg)
}

func (a *App) Connect(id, password string, rememberPassword, requireBiometric, trustNewHost bool) error {
	a.mu.Lock()
	if a.session != nil {
		_ = a.session.Close()
		a.session = nil
		a.active = ""
	}
	server, ok := a.server(id)
	logActivity := a.cfg.Preferences.LogActivity
	a.mu.Unlock()
	if !ok {
		return errors.New("server not found")
	}

	loadedSavedPassword := false
	if password == "" && server.PasswordSaved {
		if server.RequireBiometric {
			if !a.biometricAvailable {
				return fmt.Errorf("%s is unavailable; enter the password manually", biometric.Name)
			}
			if err := biometric.Authenticate("Unlock the password for " + server.Name); err != nil {
				return err
			}
		}
		var err error
		password, err = credentials.Get(server.ID)
		if err != nil {
			return fmt.Errorf("retrieve saved password: %w", err)
		}
		loadedSavedPassword = true
	}

	wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "connecting", "serverId": id})
	session, err := sshclient.Connect(server, password, trustNewHost)
	if err != nil {
		var hostKeyErr *sshclient.HostKeyVerificationError
		if !errors.As(err, &hostKeyErr) {
			wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "error", "message": err.Error()})
		}
		return err
	}

	if rememberPassword && password != "" && !loadedSavedPassword {
		if err := credentials.Set(server.ID, password); err != nil {
			_ = session.Close()
			return fmt.Errorf("save password securely: %w", err)
		}
	}
	if !rememberPassword && server.PasswordSaved {
		if err := credentials.Delete(server.ID); err != nil {
			_ = session.Close()
			return fmt.Errorf("remove saved password: %w", err)
		}
	}

	a.mu.Lock()
	for i := range a.cfg.Servers {
		if a.cfg.Servers[i].ID == id {
			if a.cfg.Servers[i].Fingerprint == "" && session.HostKeyFingerprint() != "" {
				a.cfg.Servers[i].Fingerprint = session.HostKeyFingerprint()
			}
			a.cfg.Servers[i].PasswordSaved = rememberPassword && password != ""
			a.cfg.Servers[i].RequireBiometric = a.cfg.Servers[i].PasswordSaved && requireBiometric
			break
		}
	}
	if err := a.store.Save(a.cfg); err != nil {
		a.mu.Unlock()
		_ = session.Close()
		return fmt.Errorf("save credential preferences: %w", err)
	}
	a.session = session
	a.active = id
	a.mu.Unlock()
	if logActivity {
		_ = a.store.Log(server.Name, "connect", server.User+"@"+server.Host)
	}
	wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "connected", "serverId": id})
	go a.consume(session)
	return nil
}

func (a *App) SendInput(data string) error {
	a.mu.Lock()
	session := a.session
	a.mu.Unlock()
	if session == nil {
		return errors.New("not connected")
	}
	return session.SendInput(data)
}

func (a *App) ResizeTerminal(columns, rows int) error {
	a.mu.Lock()
	session := a.session
	a.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Resize(columns, rows)
}

func (a *App) OpenInZed(id, remotePath string, newWindow, passSavedPassword bool) error {
	a.mu.Lock()
	server, ok := a.server(id)
	a.mu.Unlock()
	if !ok {
		return errors.New("server not found")
	}

	password := ""
	if passSavedPassword {
		if !server.PasswordSaved {
			return errors.New("this server does not have a saved password")
		}
		if server.RequireBiometric {
			if !a.biometricAvailable {
				return fmt.Errorf("%s is unavailable", biometric.Name)
			}
			if err := biometric.Authenticate("Unlock the password for Zed on " + server.Name); err != nil {
				return err
			}
		}
		var err error
		password, err = credentials.Get(server.ID)
		if err != nil {
			return fmt.Errorf("retrieve saved password: %w", err)
		}
	}
	return editor.OpenZed(server, remotePath, newWindow, password)
}

func (a *App) ListSSHKeys(id string) ([]sshclient.PublicKeyInfo, error) {
	a.mu.Lock()
	_, ok := a.server(id)
	a.mu.Unlock()
	if !ok {
		return nil, errors.New("server not found")
	}
	return sshclient.ListPublicKeys()
}

func (a *App) InstallSSHKey(id, publicKeyPath, password string, generate bool) (config.Config, error) {
	a.mu.Lock()
	server, ok := a.server(id)
	logActivity := a.cfg.Preferences.LogActivity
	a.mu.Unlock()
	if !ok {
		return a.cfg, errors.New("server not found")
	}

	var key sshclient.PublicKeyInfo
	var err error
	if generate {
		key, err = sshclient.GenerateServerKey(server)
		if err != nil {
			return a.cfg, err
		}
		publicKeyPath = key.Path
	}

	if password == "" && server.PasswordSaved {
		if server.RequireBiometric {
			if !a.biometricAvailable {
				return a.cfg, fmt.Errorf("%s is unavailable; enter the password manually", biometric.Name)
			}
			if err := biometric.Authenticate("Install an SSH key on " + server.Name); err != nil {
				return a.cfg, err
			}
		}
		password, err = credentials.Get(server.ID)
		if err != nil {
			return a.cfg, fmt.Errorf("retrieve saved password: %w", err)
		}
	}

	key, err = sshclient.InstallPublicKey(server, publicKeyPath, password)
	if err != nil {
		return a.cfg, err
	}

	a.mu.Lock()
	for i := range a.cfg.Servers {
		if a.cfg.Servers[i].ID == id && key.PrivatePath != "" {
			a.cfg.Servers[i].Identity = key.PrivatePath
			break
		}
	}
	if err := a.store.Save(a.cfg); err != nil {
		a.mu.Unlock()
		return a.cfg, fmt.Errorf("save server identity: %w", err)
	}
	updated := a.cfg
	a.mu.Unlock()
	if logActivity {
		_ = a.store.Log(server.Name, "key", "installed "+key.Fingerprint)
	}
	return updated, nil
}

func (a *App) SendCommand(command string) error {
	a.mu.Lock()
	session := a.session
	server, ok := a.server(a.active)
	logActivity := a.cfg.Preferences.LogActivity
	a.mu.Unlock()
	if session == nil {
		return errors.New("not connected")
	}
	if err := session.Send(command); err != nil {
		return err
	}
	if ok && logActivity {
		_ = a.store.Log(server.Name, "command", command)
	}
	return nil
}

func (a *App) Disconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.session != nil {
		_ = a.session.Close()
	}
	a.session = nil
	a.active = ""
	wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "disconnected"})
}

func (a *App) consume(session *sshclient.Session) {
	for {
		select {
		case output := <-session.Output():
			wailsruntime.EventsEmit(a.ctx, "terminal:data", output)
		case err := <-session.Done():
			a.mu.Lock()
			if a.session == session {
				a.session = nil
				a.active = ""
			}
			a.mu.Unlock()
			message := ""
			if err != nil {
				message = err.Error()
			}
			wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "disconnected", "message": message})
			return
		}
	}
}

func (a *App) server(id string) (config.Server, bool) {
	for _, server := range a.cfg.Servers {
		if server.ID == id {
			return server, true
		}
	}
	return config.Server{}, false
}

func newID() string {
	return fmt.Sprintf("%x", timeNowUnixNano())
}

var timeNowUnixNano = func() int64 {
	return time.Now().UnixNano()
}
