package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sshking/internal/biometric"
	"sshking/internal/cloudclient"
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
	sessions           map[string]*managedSession
	tunnels            map[string]*managedTunnel
	mu                 sync.Mutex
	biometricAvailable bool
	cloud              *cloudclient.Client
	cloudMu            sync.Mutex
}

type managedSession struct {
	serverID string
	session  *sshclient.Session
}

type managedTunnel struct {
	sessionID  string
	remoteHost string
	remotePort int
	forward    *sshclient.Forward
}

type TunnelInfo struct {
	ID         string `json:"id"`
	SessionID  string `json:"sessionId"`
	Local      string `json:"local"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
}

type RemoteFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"modTime"`
}

type ServerReadiness struct {
	Ready       bool   `json:"ready"`
	HasKey      bool   `json:"hasKey"`
	HasPassword bool   `json:"hasPassword"`
	HasAgent    bool   `json:"hasAgent"`
	Message     string `json:"message"`
}

type CloudSyncResult struct {
	Config    config.Config              `json:"config"`
	Workspace cloudclient.Workspace      `json:"workspace"`
	Readiness map[string]ServerReadiness `json:"readiness"`
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
	return &App{
		store:              store,
		cfg:                cfg,
		sessions:           make(map[string]*managedSession),
		tunnels:            make(map[string]*managedTunnel),
		biometricAvailable: biometric.Available(),
		cloud:              cloudclient.New(),
	}, nil
}

func (a *App) GetCloudState(cloudURL string) (cloudclient.State, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return a.cloud.Status(ctx, cloudURL)
}

func (a *App) LoginCloud(cloudURL, provider string) (cloudclient.State, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 4*time.Minute)
	defer cancel()
	state, err := a.cloud.Login(ctx, cloudURL, provider, func(target string) {
		wailsruntime.BrowserOpenURL(a.ctx, target)
	})
	if err != nil {
		return cloudclient.State{}, err
	}
	a.mu.Lock()
	a.cfg.Preferences.CloudURL = strings.TrimRight(strings.TrimSpace(cloudURL), "/")
	err = a.store.Save(a.cfg)
	a.mu.Unlock()
	return state, err
}

func (a *App) LogoutCloud(cloudURL string) error {
	return a.cloud.Logout(cloudURL)
}

func (a *App) GetServerReadiness() map[string]ServerReadiness {
	a.mu.Lock()
	servers := append([]config.Server(nil), a.cfg.Servers...)
	a.mu.Unlock()
	return serverReadiness(servers)
}

func (a *App) SyncCloudWorkspace(cloudURL string, tabs []cloudclient.CloudTab, deleteTabIDs, deleteServerIDs, deleteTeamIDs []string) (CloudSyncResult, error) {
	a.cloudMu.Lock()
	defer a.cloudMu.Unlock()
	a.mu.Lock()
	local := a.cfg
	a.mu.Unlock()
	patch := cloudclient.WorkspacePatch{Tabs: tabs, DeleteTabIDs: deleteTabIDs, DeleteServerIDs: deleteServerIDs, DeleteTeamIDs: deleteTeamIDs}
	for _, server := range local.Servers {
		patch.Servers = append(patch.Servers, cloudclient.CloudServer{
			ID: server.ID, TeamID: server.TeamID, Name: server.Name, Group: server.Group,
			Host: server.Host, Port: server.Port, User: server.User, Shell: server.Shell,
			UseTmux: server.UseTmux, TmuxSession: server.TmuxSession, JumpServerID: server.JumpServerID,
			Fingerprint: server.Fingerprint, Favorite: server.Favorite,
		})
	}
	for _, team := range local.Teams {
		patch.Teams = append(patch.Teams, cloudclient.CloudTeam{ID: team.ID, Name: team.Name})
	}
	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	workspace, err := a.cloud.SyncWorkspace(ctx, cloudURL, patch)
	if err != nil {
		return CloudSyncResult{}, err
	}
	localByID := make(map[string]config.Server, len(local.Servers))
	for _, server := range local.Servers {
		localByID[server.ID] = server
	}
	merged := make([]config.Server, 0, len(workspace.Servers))
	for _, remote := range workspace.Servers {
		server := config.Server{ID: remote.ID, TeamID: remote.TeamID, Name: remote.Name, Group: remote.Group, Host: remote.Host, Port: remote.Port, User: remote.User, Shell: remote.Shell, UseTmux: remote.UseTmux, TmuxSession: remote.TmuxSession, JumpServerID: remote.JumpServerID, Fingerprint: remote.Fingerprint, Favorite: remote.Favorite}
		if device, exists := localByID[remote.ID]; exists {
			server.Identity = device.Identity
			server.PasswordSaved = device.PasswordSaved
			server.RequireBiometric = device.RequireBiometric
		}
		merged = append(merged, server)
	}
	teams := make([]config.Team, 0, len(workspace.Teams))
	for _, team := range workspace.Teams {
		teams = append(teams, config.Team{ID: team.ID, Name: team.Name})
	}
	a.mu.Lock()
	a.cfg.Servers = merged
	a.cfg.Teams = teams
	saveErr := a.store.Save(a.cfg)
	resultConfig := a.cfg
	a.mu.Unlock()
	if saveErr != nil {
		return CloudSyncResult{}, saveErr
	}
	return CloudSyncResult{Config: resultConfig, Workspace: workspace, Readiness: serverReadiness(merged)}, nil
}

func serverReadiness(servers []config.Server) map[string]ServerReadiness {
	result := make(map[string]ServerReadiness, len(servers))
	agentReady := sshclient.AgentHasKeys()
	for _, server := range servers {
		status := ServerReadiness{HasAgent: agentReady}
		if strings.TrimSpace(server.Identity) != "" {
			identity := server.Identity
			if identity == "~" || strings.HasPrefix(identity, "~/") || strings.HasPrefix(identity, `~\`) {
				if home, err := os.UserHomeDir(); err == nil {
					identity = filepath.Join(home, strings.TrimLeft(identity[1:], `/\`))
				}
			}
			if info, err := os.Stat(identity); err == nil && !info.IsDir() {
				status.HasKey = true
			} else {
				status.Message = "Private key is missing on this device"
			}
		}
		if server.PasswordSaved {
			if _, err := credentials.Get(server.ID); err == nil {
				status.HasPassword = true
			}
		}
		status.Ready = status.HasKey || status.HasPassword || (strings.TrimSpace(server.Identity) == "" && status.HasAgent)
		if !status.Ready && status.Message == "" {
			status.Message = "Add a password or SSH key on this device"
		}
		result[server.ID] = status
	}
	return result
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, managed := range a.sessions {
		_ = managed.session.Close()
	}
	for _, tunnel := range a.tunnels {
		_ = tunnel.forward.Close()
	}
	a.sessions = make(map[string]*managedSession)
	a.tunnels = make(map[string]*managedTunnel)
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
	if server.TeamID != "" && !a.teamExists(server.TeamID) {
		return a.cfg, errors.New("selected team was not found")
	}
	server.TmuxSession = strings.TrimSpace(server.TmuxSession)
	if server.UseTmux {
		if server.TmuxSession == "" {
			server.TmuxSession = "sshking"
		}
		if !validTmuxSessionName(server.TmuxSession) {
			return a.cfg, errors.New("tmux session name may contain only letters, numbers, hyphens, and underscores")
		}
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

func (a *App) SaveTeam(team config.Team) (config.Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	team.Name = strings.TrimSpace(team.Name)
	if team.Name == "" {
		return a.cfg, errors.New("team name is required")
	}
	for _, existing := range a.cfg.Teams {
		if existing.ID != team.ID && strings.EqualFold(existing.Name, team.Name) {
			return a.cfg, errors.New("a team with this name already exists")
		}
	}
	if team.ID == "" {
		team.ID = newID()
		a.cfg.Teams = append(a.cfg.Teams, team)
	} else {
		found := false
		for i := range a.cfg.Teams {
			if a.cfg.Teams[i].ID == team.ID {
				a.cfg.Teams[i] = team
				found = true
				break
			}
		}
		if !found {
			return a.cfg, errors.New("team not found")
		}
	}
	return a.cfg, a.store.Save(a.cfg)
}

func (a *App) DeleteTeam(id string) (config.Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.cfg.Teams {
		if a.cfg.Teams[i].ID != id {
			continue
		}
		a.cfg.Teams = append(a.cfg.Teams[:i], a.cfg.Teams[i+1:]...)
		for serverIndex := range a.cfg.Servers {
			if a.cfg.Servers[serverIndex].TeamID == id {
				a.cfg.Servers[serverIndex].TeamID = ""
			}
		}
		return a.cfg, a.store.Save(a.cfg)
	}
	return a.cfg, errors.New("team not found")
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
			for sessionID, managed := range a.sessions {
				if managed.serverID == id {
					_ = managed.session.Close()
					a.closeSessionTunnelsLocked(sessionID)
					delete(a.sessions, sessionID)
				}
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
	if preferences.Theme != "light" && preferences.Theme != "black" && preferences.Theme != "glass" {
		return a.cfg, errors.New("theme must be light, black, or glass")
	}
	if preferences.UIScale < 80 || preferences.UIScale > 140 {
		return a.cfg, errors.New("interface scale must be between 80 and 140 percent")
	}
	if preferences.TerminalFontSize < 10 || preferences.TerminalFontSize > 28 {
		return a.cfg, errors.New("terminal font size must be between 10 and 28")
	}
	if preferences.TerminalLineHeight < 100 || preferences.TerminalLineHeight > 200 {
		return a.cfg, errors.New("terminal line height must be between 100 and 200 percent")
	}
	switch preferences.TerminalFontFamily {
	case "system-mono", "cascadia", "jetbrains", "source-code":
	default:
		return a.cfg, errors.New("unsupported terminal font family")
	}
	a.cfg.Preferences = preferences
	return a.cfg, a.store.Save(a.cfg)
}

func (a *App) GetSessionTranscript(sessionID string) (string, error) {
	a.mu.Lock()
	enabled := a.cfg.Preferences.PersistTerminalHistory
	a.mu.Unlock()
	if !enabled {
		return "", nil
	}
	return a.store.Transcript(sessionID)
}

func (a *App) ClearSessionTranscript(sessionID string) error {
	return a.store.ClearTranscript(sessionID)
}

func (a *App) ClearTerminalHistory() error {
	return a.store.ClearTranscripts()
}

func (a *App) ImportSSHConfig(path string) (config.Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	imported, err := config.ImportOpenSSH(path, a.cfg.Preferences)
	if err != nil {
		return a.cfg, fmt.Errorf("import SSH config: %w", err)
	}
	for _, candidate := range imported {
		duplicate := false
		for _, existing := range a.cfg.Servers {
			if existing.Name == candidate.Name || (existing.Host == candidate.Host && existing.User == candidate.User && existing.Port == candidate.Port) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		candidate.ID = newID()
		a.cfg.Servers = append(a.cfg.Servers, candidate)
	}
	if err := a.store.Save(a.cfg); err != nil {
		return a.cfg, err
	}
	return a.cfg, nil
}

func (a *App) GetActivity(limit int) ([]string, error) {
	return a.store.Activity(limit)
}

func (a *App) Connect(sessionID, id, password string, rememberPassword, requireBiometric, trustNewHost bool) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}
	a.mu.Lock()
	if existing := a.sessions[sessionID]; existing != nil {
		_ = existing.session.Close()
		delete(a.sessions, sessionID)
	}
	server, ok := a.server(id)
	var jumpServer config.Server
	var jumpOK bool
	if ok && server.JumpServerID != "" {
		jumpServer, jumpOK = a.server(server.JumpServerID)
	}
	logActivity := a.cfg.Preferences.LogActivity
	a.mu.Unlock()
	if !ok {
		return errors.New("server not found")
	}
	if server.JumpServerID != "" && !jumpOK {
		return errors.New("configured jump host was not found")
	}
	if server.UseTmux {
		server.TmuxSession = tmuxSessionForTab(server.TmuxSession, sessionID)
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

	wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "connecting", "sessionId": sessionID, "serverId": id})
	var session *sshclient.Session
	var err error
	if jumpOK {
		jumpPassword := ""
		if jumpServer.PasswordSaved {
			if jumpServer.RequireBiometric {
				if !a.biometricAvailable {
					return fmt.Errorf("%s is unavailable for jump host %s", biometric.Name, jumpServer.Name)
				}
				if err := biometric.Authenticate("Unlock the jump host password for " + jumpServer.Name); err != nil {
					return err
				}
			}
			jumpPassword, err = credentials.Get(jumpServer.ID)
			if err != nil {
				return fmt.Errorf("retrieve jump host password: %w", err)
			}
		}
		session, err = sshclient.ConnectVia(server, password, jumpServer, jumpPassword, trustNewHost)
	} else {
		session, err = sshclient.Connect(server, password, trustNewHost)
	}
	if err != nil {
		var hostKeyErr *sshclient.HostKeyVerificationError
		if !errors.As(err, &hostKeyErr) {
			wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "error", "sessionId": sessionID, "serverId": id, "message": err.Error()})
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
	a.sessions[sessionID] = &managedSession{serverID: id, session: session}
	a.mu.Unlock()
	if logActivity {
		_ = a.store.Log(server.Name, "connect", server.User+"@"+server.Host)
	}
	wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "connected", "sessionId": sessionID, "serverId": id})
	go a.consume(sessionID, session)
	return nil
}

func (a *App) SendInput(sessionID, data string) error {
	a.mu.Lock()
	managed := a.sessions[sessionID]
	a.mu.Unlock()
	if managed == nil {
		return errors.New("not connected")
	}
	return managed.session.SendInput(data)
}

func (a *App) ResizeTerminal(sessionID string, columns, rows int) error {
	a.mu.Lock()
	managed := a.sessions[sessionID]
	a.mu.Unlock()
	if managed == nil {
		return nil
	}
	return managed.session.Resize(columns, rows)
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

func (a *App) SendCommand(sessionID, command string) error {
	a.mu.Lock()
	managed := a.sessions[sessionID]
	logActivity := a.cfg.Preferences.LogActivity
	var server config.Server
	var ok bool
	if managed != nil {
		server, ok = a.server(managed.serverID)
	}
	a.mu.Unlock()
	if managed == nil {
		return errors.New("not connected")
	}
	if err := managed.session.Send(command); err != nil {
		return err
	}
	if ok && logActivity {
		_ = a.store.Log(server.Name, "command", command)
	}
	return nil
}

func (a *App) Disconnect(sessionID string) {
	a.mu.Lock()
	managed := a.sessions[sessionID]
	if managed != nil {
		delete(a.sessions, sessionID)
	}
	a.closeSessionTunnelsLocked(sessionID)
	a.mu.Unlock()
	if managed != nil {
		_ = managed.session.Close()
	}
	wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "disconnected", "sessionId": sessionID})
}

func (a *App) StartLocalTunnel(sessionID string, localPort int, remoteHost string, remotePort int) (TunnelInfo, error) {
	if localPort < 0 || localPort > 65535 || remotePort < 1 || remotePort > 65535 {
		return TunnelInfo{}, errors.New("invalid tunnel port")
	}
	if remoteHost == "" {
		return TunnelInfo{}, errors.New("remote host is required")
	}
	managed, err := a.managedSession(sessionID)
	if err != nil {
		return TunnelInfo{}, err
	}
	forward, err := managed.session.ForwardLocal(
		sshclient.TunnelAddress("127.0.0.1", localPort),
		sshclient.TunnelAddress(remoteHost, remotePort),
	)
	if err != nil {
		return TunnelInfo{}, fmt.Errorf("start local tunnel: %w", err)
	}
	id := newID()
	tunnel := &managedTunnel{sessionID: sessionID, remoteHost: remoteHost, remotePort: remotePort, forward: forward}
	a.mu.Lock()
	a.tunnels[id] = tunnel
	a.mu.Unlock()
	return TunnelInfo{ID: id, SessionID: sessionID, Local: forward.Address(), RemoteHost: remoteHost, RemotePort: remotePort}, nil
}

func (a *App) StopTunnel(id string) error {
	a.mu.Lock()
	tunnel := a.tunnels[id]
	delete(a.tunnels, id)
	a.mu.Unlock()
	if tunnel == nil {
		return errors.New("tunnel not found")
	}
	return tunnel.forward.Close()
}

func (a *App) closeSessionTunnelsLocked(sessionID string) {
	for id, tunnel := range a.tunnels {
		if tunnel.sessionID == sessionID {
			_ = tunnel.forward.Close()
			delete(a.tunnels, id)
		}
	}
}

func (a *App) ListRemoteFiles(sessionID, remotePath string) ([]RemoteFile, error) {
	managed, err := a.managedSession(sessionID)
	if err != nil {
		return nil, err
	}
	client, err := managed.session.NewSFTPClient()
	if err != nil {
		return nil, fmt.Errorf("start SFTP: %w", err)
	}
	defer client.Close()
	if remotePath == "" || remotePath == "~" {
		if remotePath, err = client.Getwd(); err != nil {
			return nil, err
		}
	}
	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return nil, err
	}
	files := make([]RemoteFile, 0, len(entries))
	for _, entry := range entries {
		files = append(files, RemoteFile{
			Name: entry.Name(), Path: path.Join(remotePath, entry.Name()), IsDir: entry.IsDir(),
			Size: entry.Size(), Mode: entry.Mode().String(), ModTime: entry.ModTime().Unix(),
		})
	}
	return files, nil
}

func (a *App) DownloadRemoteFile(sessionID, remotePath string) (string, error) {
	managed, err := a.managedSession(sessionID)
	if err != nil {
		return "", err
	}
	destination, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{DefaultFilename: path.Base(remotePath)})
	if err != nil || destination == "" {
		return destination, err
	}
	client, err := managed.session.NewSFTPClient()
	if err != nil {
		return "", err
	}
	defer client.Close()
	source, err := client.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	target, err := os.Create(destination)
	if err != nil {
		return "", err
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return "", err
	}
	return destination, nil
}

func (a *App) UploadRemoteFile(sessionID, remoteDirectory string) (string, error) {
	managed, err := a.managedSession(sessionID)
	if err != nil {
		return "", err
	}
	sourcePath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Upload file"})
	if err != nil || sourcePath == "" {
		return "", err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	client, err := managed.session.NewSFTPClient()
	if err != nil {
		return "", err
	}
	defer client.Close()
	if remoteDirectory == "" || remoteDirectory == "~" {
		if remoteDirectory, err = client.Getwd(); err != nil {
			return "", err
		}
	}
	remotePath := path.Join(remoteDirectory, filepath.Base(sourcePath))
	target, err := client.Create(remotePath)
	if err != nil {
		return "", err
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return "", err
	}
	return remotePath, nil
}

func (a *App) managedSession(sessionID string) (*managedSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	managed := a.sessions[sessionID]
	if managed == nil {
		return nil, errors.New("session is not connected")
	}
	return managed, nil
}

func (a *App) consume(sessionID string, session *sshclient.Session) {
	for {
		select {
		case output := <-session.Output():
			a.mu.Lock()
			managed := a.sessions[sessionID]
			current := managed != nil && managed.session == session
			persistTranscript := a.cfg.Preferences.PersistTerminalHistory
			a.mu.Unlock()
			if !current {
				return
			}
			if persistTranscript {
				_ = a.store.AppendTranscript(sessionID, output)
			}
			wailsruntime.EventsEmit(a.ctx, "terminal:data", map[string]any{"sessionId": sessionID, "data": output})
		case err := <-session.Done():
			a.mu.Lock()
			current := false
			if managed := a.sessions[sessionID]; managed != nil && managed.session == session {
				current = true
				delete(a.sessions, sessionID)
				a.closeSessionTunnelsLocked(sessionID)
			}
			a.mu.Unlock()
			if !current {
				return
			}
			message := ""
			if err != nil {
				message = err.Error()
			}
			wailsruntime.EventsEmit(a.ctx, "terminal:status", map[string]any{"state": "disconnected", "sessionId": sessionID, "message": message})
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

func (a *App) teamExists(id string) bool {
	for _, team := range a.cfg.Teams {
		if team.ID == id {
			return true
		}
	}
	return false
}

func newID() string {
	return fmt.Sprintf("%x", timeNowUnixNano())
}

func validTmuxSessionName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func tmuxSessionForTab(base, sessionID string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "sshking"
	}
	sum := sha256.Sum256([]byte(sessionID))
	suffix := fmt.Sprintf("%x", sum[:6])
	maxBaseLength := 64 - len(suffix) - 1
	if len(base) > maxBaseLength {
		base = base[:maxBaseLength]
	}
	return base + "-" + suffix
}

var timeNowUnixNano = func() int64 {
	return time.Now().UnixNano()
}
