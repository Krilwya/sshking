package cloudapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Server struct {
	cfg       Config
	store     *store
	providers map[string]*provider
	mux       *http.ServeMux
}

func New(ctx context.Context, cfg Config) (*Server, error) {
	store, err := newStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, store: store, providers: newProviders(cfg), mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func (s *Server) Close()                { s.store.close() }
func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /v1/auth/providers", s.providerStatus)
	s.mux.HandleFunc("POST /v1/auth/start", s.authStart)
	s.mux.HandleFunc("GET /v1/auth/callback/{provider}", s.authCallback)
	s.mux.HandleFunc("POST /v1/auth/callback/{provider}", s.authCallback)
	s.mux.HandleFunc("POST /v1/auth/token", s.authToken)
	s.mux.HandleFunc("POST /v1/auth/refresh", s.authRefresh)
	s.mux.HandleFunc("GET /v1/me", s.me)
	s.mux.HandleFunc("GET /v1/workspace", s.getWorkspace)
	s.mux.HandleFunc("POST /v1/workspace/sync", s.syncWorkspace)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "providers": s.enabledProviders()})
}

func (s *Server) enabledProviders() map[string]bool {
	return map[string]bool{"google": s.providers["google"] != nil, "apple": s.providers["apple"] != nil}
}
func (s *Server) providerStatus(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	writeJSON(w, http.StatusOK, s.enabledProviders())
}

func (s *Server) authStart(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var input struct {
		Provider      string `json:"provider"`
		RedirectURI   string `json:"redirectUri"`
		CodeChallenge string `json:"codeChallenge"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	p := s.providers[input.Provider]
	if p == nil {
		writeError(w, http.StatusBadRequest, "provider is not configured")
		return
	}
	if err := validateLoopbackRedirect(input.RedirectURI); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !validChallenge(input.CodeChallenge) {
		writeError(w, http.StatusBadRequest, "invalid PKCE code challenge")
		return
	}
	state, err := randomToken(32)
	if err != nil {
		writeError(w, 500, "could not start login")
		return
	}
	if err := s.store.createFlow(r.Context(), tokenHash(state), input.Provider, input.RedirectURI, input.CodeChallenge, time.Now().Add(10*time.Minute)); err != nil {
		log.Printf("create auth flow: %v", err)
		writeError(w, 500, "could not start login")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authorizationUrl": p.authorizationURL(state), "state": state})
}

func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	if err := r.ParseForm(); err != nil {
		writeError(w, 400, "invalid callback")
		return
	}
	state := r.Form.Get("state")
	if state == "" {
		writeError(w, 400, "missing state")
		return
	}
	flow, err := s.store.consumeFlow(r.Context(), tokenHash(state))
	if err != nil {
		writeError(w, 400, "login request expired or already used")
		return
	}
	if flow.Provider != r.PathValue("provider") {
		http.Redirect(w, r, appendRedirect(flow.RedirectURI, url.Values{"error": {"provider_mismatch"}, "state": {state}}), http.StatusFound)
		return
	}
	if providerError := r.Form.Get("error"); providerError != "" {
		http.Redirect(w, r, appendRedirect(flow.RedirectURI, url.Values{"error": {providerError}, "state": {state}}), http.StatusFound)
		return
	}
	p := s.providers[flow.Provider]
	if p == nil {
		writeError(w, 400, "provider is not configured")
		return
	}
	claims, err := p.exchange(r.Context(), r.Form.Get("code"))
	if err != nil {
		log.Printf("oauth callback: %v", err)
		http.Redirect(w, r, appendRedirect(flow.RedirectURI, url.Values{"error": {"identity_verification_failed"}, "state": {state}}), http.StatusFound)
		return
	}
	u, err := s.store.upsertIdentity(r.Context(), flow.Provider, claims.Subject, claims.Email, claims.Name, claims.Picture)
	if err != nil {
		log.Printf("upsert identity: %v", err)
		writeError(w, 500, "could not save identity")
		return
	}
	code, err := randomToken(32)
	if err == nil {
		err = s.store.createLoginCode(r.Context(), tokenHash(code), u.ID, flow.CodeChallenge, state, time.Now().Add(s.cfg.LoginCodeTTL))
	}
	if err != nil {
		log.Printf("create login code: %v", err)
		writeError(w, 500, "could not complete login")
		return
	}
	http.Redirect(w, r, appendRedirect(flow.RedirectURI, url.Values{"code": {code}, "state": {state}}), http.StatusFound)
}

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
	User         user   `json:"user"`
}

func (s *Server) authToken(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var input struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"codeVerifier"`
		DeviceName   string `json:"deviceName"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	userID, err := s.store.consumeLoginCode(r.Context(), tokenHash(input.Code), pkceChallenge(input.CodeVerifier))
	if err != nil {
		writeError(w, 400, "login code expired, already used, or PKCE verification failed")
		return
	}
	u := user{ID: userID}
	if err := s.issueSession(r.Context(), u, strings.TrimSpace(input.DeviceName), w); err != nil {
		log.Printf("issue session: %v", err)
		writeError(w, 500, "could not create device session")
	}
}

func (s *Server) authRefresh(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var input struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	access, _ := randomToken(32)
	refresh, _ := randomToken(32)
	now := time.Now()
	u, err := s.store.rotateSession(r.Context(), tokenHash(input.RefreshToken), tokenHash(access), tokenHash(refresh), now.Add(s.cfg.AccessTokenTTL), now.Add(s.cfg.RefreshTokenTTL))
	if err != nil {
		writeError(w, 401, "invalid or expired refresh token")
		return
	}
	writeJSON(w, 200, tokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.cfg.AccessTokenTTL.Seconds()), User: u})
}

func (s *Server) issueSession(ctx context.Context, u user, device string, w http.ResponseWriter) error {
	access, err := randomToken(32)
	if err != nil {
		return err
	}
	refresh, err := randomToken(32)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := s.store.createSession(ctx, u.ID, device, tokenHash(access), tokenHash(refresh), now.Add(s.cfg.AccessTokenTTL), now.Add(s.cfg.RefreshTokenTTL)); err != nil {
		return err
	}
	stored, err := s.store.userByAccess(ctx, tokenHash(access))
	if err != nil {
		return err
	}
	writeJSON(w, 200, tokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.cfg.AccessTokenTTL.Seconds()), User: stored})
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, u)
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (user, bool) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeError(w, 401, "missing bearer token")
		return user{}, false
	}
	u, err := s.store.userByAccess(r.Context(), tokenHash(parts[1]))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 401, "invalid or expired access token")
		return user{}, false
	}
	if err != nil {
		writeError(w, 500, "could not load account")
		return user{}, false
	}
	return u, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		writeError(w, 400, "invalid JSON body")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
