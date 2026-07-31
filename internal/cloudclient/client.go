package cloudclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const keyringService = "SSHKing Cloud"

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	AvatarURL   string `json:"avatarUrl"`
}

type State struct {
	CloudURL  string          `json:"cloudUrl"`
	SignedIn  bool            `json:"signedIn"`
	User      User            `json:"user"`
	Providers map[string]bool `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

type session struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	User         User      `json:"user"`
}

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	User         User   `json:"user"`
}

type Client struct {
	HTTP *http.Client
	mu   sync.Mutex
}

type CloudServer struct {
	ID           string `json:"id"`
	TeamID       string `json:"teamId,omitempty"`
	Name         string `json:"name"`
	Group        string `json:"group,omitempty"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Shell        string `json:"shell"`
	UseTmux      bool   `json:"useTmux,omitempty"`
	TmuxSession  string `json:"tmuxSession,omitempty"`
	JumpServerID string `json:"jumpServerId,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	Favorite     bool   `json:"favorite,omitempty"`
}

type CloudTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CloudTab struct {
	ID          string `json:"id"`
	ServerID    string `json:"serverId"`
	Title       string `json:"title"`
	ManualTitle bool   `json:"manualTitle,omitempty"`
	Restore     bool   `json:"restore,omitempty"`
	LastPath    string `json:"lastPath,omitempty"`
	Position    int    `json:"position,omitempty"`
}

type WorkspacePatch struct {
	Servers         []CloudServer `json:"servers"`
	Teams           []CloudTeam   `json:"teams"`
	Tabs            []CloudTab    `json:"tabs"`
	DeleteServerIDs []string      `json:"deleteServerIds"`
	DeleteTeamIDs   []string      `json:"deleteTeamIds"`
	DeleteTabIDs    []string      `json:"deleteTabIds"`
}

type Workspace struct {
	Revision  int64         `json:"revision"`
	Servers   []CloudServer `json:"servers"`
	Teams     []CloudTeam   `json:"teams"`
	Tabs      []CloudTab    `json:"tabs"`
	UpdatedAt string        `json:"updatedAt"`
}

func New() *Client { return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}} }

func normalizeBase(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("enter a valid cloud URL")
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("cloud URL must not include a path, query, or fragment")
	}
	if u.Scheme != "https" {
		host := u.Hostname()
		ip := net.ParseIP(host)
		if u.Scheme != "http" || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
			return "", errors.New("cloud URL must use HTTPS")
		}
	}
	return raw, nil
}

func (c *Client) Status(ctx context.Context, raw string) (State, error) {
	base, err := normalizeBase(raw)
	if err != nil {
		return State{CloudURL: raw}, err
	}
	result := State{CloudURL: base, Providers: map[string]bool{}}
	if err := c.getJSON(ctx, base+"/v1/auth/providers", "", &result.Providers); err != nil {
		return result, err
	}
	sess, err := loadSession(base)
	if errors.Is(err, keyring.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if time.Now().After(sess.ExpiresAt.Add(-30 * time.Second)) {
		sess, err = c.refresh(ctx, base, sess.RefreshToken)
		if err != nil {
			_ = deleteSession(base)
			return result, nil
		}
		if err := saveSession(base, sess); err != nil {
			return result, err
		}
	}
	result.SignedIn = true
	result.User = sess.User
	return result, nil
}

func (c *Client) Login(ctx context.Context, raw, provider string, openBrowser func(string)) (State, error) {
	base, err := normalizeBase(raw)
	if err != nil {
		return State{}, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return State{}, err
	}
	defer listener.Close()
	verifier, err := randomToken(32)
	if err != nil {
		return State{}, err
	}
	challengeHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])
	redirectURI := "http://" + listener.Addr().String() + "/oauth/callback"
	var start struct {
		AuthorizationURL string `json:"authorizationUrl"`
		State            string `json:"state"`
	}
	if err := c.postJSON(ctx, base+"/v1/auth/start", map[string]string{"provider": provider, "redirectUri": redirectURI, "codeChallenge": challenge}, &start); err != nil {
		return State{}, err
	}
	if start.AuthorizationURL == "" || start.State == "" {
		return State{}, errors.New("cloud returned an invalid login request")
	}
	type callbackResult struct{ code, state, providerError string }
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		result := callbackResult{code: r.URL.Query().Get("code"), state: r.URL.Query().Get("state"), providerError: r.URL.Query().Get("error")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		message := "Login complete. You can return to SSHKing."
		if result.providerError != "" {
			message = "Login failed: " + html.EscapeString(result.providerError)
		}
		_, _ = io.WriteString(w, "<!doctype html><meta name=viewport content='width=device-width'><title>SSHKing</title><style>body{font:16px system-ui;background:#10121b;color:#eef;padding:48px}main{max-width:520px;margin:auto}</style><main><h1>SSHKing</h1><p>"+message+"</p></main>")
		select {
		case resultCh <- result:
		default:
		}
	})
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())
	openBrowser(start.AuthorizationURL)
	var callback callbackResult
	select {
	case callback = <-resultCh:
	case <-ctx.Done():
		return State{}, ctx.Err()
	case <-time.After(3 * time.Minute):
		return State{}, errors.New("login timed out")
	}
	if callback.providerError != "" {
		return State{}, fmt.Errorf("provider login failed: %s", callback.providerError)
	}
	if callback.state != start.State || callback.code == "" {
		return State{}, errors.New("login callback could not be verified")
	}
	var token tokenResponse
	deviceName, _ := osHostname()
	if err := c.postJSON(ctx, base+"/v1/auth/token", map[string]string{"code": callback.code, "codeVerifier": verifier, "deviceName": deviceName}, &token); err != nil {
		return State{}, err
	}
	sess := session{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, ExpiresAt: time.Now().Add(time.Duration(token.ExpiresIn) * time.Second), User: token.User}
	if err := saveSession(base, sess); err != nil {
		return State{}, fmt.Errorf("save cloud session securely: %w", err)
	}
	return c.Status(ctx, base)
}

func (c *Client) Logout(raw string) error {
	base, err := normalizeBase(raw)
	if err != nil {
		return err
	}
	return deleteSession(base)
}

func (c *Client) Workspace(ctx context.Context, raw string) (Workspace, error) {
	base, sess, err := c.authorizedSession(ctx, raw)
	if err != nil {
		return Workspace{}, err
	}
	var workspace Workspace
	if err := c.getJSON(ctx, base+"/v1/workspace", sess.AccessToken, &workspace); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

func (c *Client) SyncWorkspace(ctx context.Context, raw string, patch WorkspacePatch) (Workspace, error) {
	base, sess, err := c.authorizedSession(ctx, raw)
	if err != nil {
		return Workspace{}, err
	}
	var workspace Workspace
	if err := c.postJSONAuthorized(ctx, base+"/v1/workspace/sync", sess.AccessToken, patch, &workspace); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

func (c *Client) authorizedSession(ctx context.Context, raw string) (string, session, error) {
	base, err := normalizeBase(raw)
	if err != nil {
		return "", session{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := loadSession(base)
	if err != nil {
		return "", session{}, errors.New("sign in to SSHKing Cloud first")
	}
	if time.Now().After(sess.ExpiresAt.Add(-30 * time.Second)) {
		sess, err = c.refresh(ctx, base, sess.RefreshToken)
		if err != nil {
			return "", session{}, err
		}
		if err = saveSession(base, sess); err != nil {
			return "", session{}, err
		}
	}
	return base, sess, nil
}

func (c *Client) refresh(ctx context.Context, base, refreshToken string) (session, error) {
	var token tokenResponse
	if err := c.postJSON(ctx, base+"/v1/auth/refresh", map[string]string{"refreshToken": refreshToken}, &token); err != nil {
		return session{}, err
	}
	return session{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, ExpiresAt: time.Now().Add(time.Duration(token.ExpiresIn) * time.Second), User: token.User}, nil
}
func (c *Client) getJSON(ctx context.Context, endpoint, bearer string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.do(req, target)
}
func (c *Client) postJSON(ctx context.Context, endpoint string, input, target any) error {
	return c.postJSONAuthorized(ctx, endpoint, "", input, target)
}
func (c *Client) postJSONAuthorized(ctx context.Context, endpoint, bearer string, input, target any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.do(req, target)
}
func (c *Client) do(req *http.Request, target any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &payload)
		if payload.Error == "" {
			payload.Error = resp.Status
		}
		return errors.New(payload.Error)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(target)
}
func randomToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func account(base string) string {
	h := sha256.Sum256([]byte(base))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func loadSession(base string) (session, error) {
	raw, err := keyring.Get(keyringService, account(base))
	if err != nil {
		return session{}, err
	}
	var value session
	err = json.Unmarshal([]byte(raw), &value)
	return value, err
}
func saveSession(base string, value session) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, account(base), string(raw))
}
func deleteSession(base string) error {
	err := keyring.Delete(keyringService, account(base))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
func osHostname() (string, error) {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "SSHKing desktop", nil
	}
	return name, nil
}
