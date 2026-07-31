package cloudapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type provider struct {
	name      string
	clientID  string
	oauth     oauth2.Config
	verifier  *oidc.IDTokenVerifier
	extraAuth []oauth2.AuthCodeOption
}

type identityClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func newProviders(cfg Config) map[string]*provider {
	providers := map[string]*provider{}
	if cfg.Google.Enabled(cfg.PublicURL) {
		providers["google"] = &provider{
			name: "google", clientID: cfg.Google.ClientID,
			oauth:    oauth2.Config{ClientID: cfg.Google.ClientID, ClientSecret: cfg.Google.ClientSecret, RedirectURL: cfg.PublicURL + "/v1/auth/callback/google", Scopes: []string{oidc.ScopeOpenID, "profile", "email"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}},
			verifier: oidc.NewVerifier("https://accounts.google.com", oidc.NewRemoteKeySet(context.Background(), "https://www.googleapis.com/oauth2/v3/certs"), &oidc.Config{ClientID: cfg.Google.ClientID}),
		}
	}
	if cfg.Apple.Enabled(cfg.PublicURL) {
		providers["apple"] = &provider{
			name: "apple", clientID: cfg.Apple.ClientID,
			oauth:     oauth2.Config{ClientID: cfg.Apple.ClientID, ClientSecret: cfg.Apple.ClientSecret, RedirectURL: cfg.PublicURL + "/v1/auth/callback/apple", Scopes: []string{"name", "email"}, Endpoint: oauth2.Endpoint{AuthURL: "https://appleid.apple.com/auth/authorize", TokenURL: "https://appleid.apple.com/auth/token"}},
			verifier:  oidc.NewVerifier("https://appleid.apple.com", oidc.NewRemoteKeySet(context.Background(), "https://appleid.apple.com/auth/keys"), &oidc.Config{ClientID: cfg.Apple.ClientID}),
			extraAuth: []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("response_mode", "form_post")},
		}
	}
	return providers
}

func randomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func tokenHash(value string) []byte { h := sha256.Sum256([]byte(value)); return h[:] }

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func validChallenge(challenge string) bool {
	if len(challenge) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(decoded) == sha256.Size
}

func validateLoopbackRedirect(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid redirect URI")
	}
	if u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("redirect URI must be a plain HTTP loopback URL")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || port == "" {
		return errors.New("redirect URI must include a loopback port")
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() {
		return errors.New("redirect URI must use a loopback address")
	}
	if u.Path != "/oauth/callback" {
		return errors.New("redirect URI path must be /oauth/callback")
	}
	return nil
}

func (p *provider) authorizationURL(state string) string {
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	opts = append(opts, p.extraAuth...)
	return p.oauth.AuthCodeURL(state, opts...)
}

func (p *provider) exchange(ctx context.Context, code string) (identityClaims, error) {
	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return identityClaims{}, fmt.Errorf("provider token exchange: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return identityClaims{}, errors.New("provider did not return an ID token")
	}
	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return identityClaims{}, fmt.Errorf("verify provider identity: %w", err)
	}
	var claims identityClaims
	if err := idToken.Claims(&claims); err != nil {
		return identityClaims{}, err
	}
	if claims.Subject == "" {
		return identityClaims{}, errors.New("provider identity has no subject")
	}
	if claims.Name == "" {
		claims.Name = claims.Email
	}
	return claims, nil
}

func appendRedirect(raw string, values url.Values) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for key, list := range values {
		for _, v := range list {
			q.Add(key, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

var _ = time.Now
