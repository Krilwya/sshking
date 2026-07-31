package cloudapi

import (
	"errors"
	"os"
	"strings"
	"time"
)

type ProviderConfig struct {
	ClientID     string
	ClientSecret string
}

func (p ProviderConfig) Enabled(publicURL string) bool {
	return publicURL != "" && p.ClientID != "" && p.ClientSecret != ""
}

type Config struct {
	ListenAddr      string
	DatabaseURL     string
	PublicURL       string
	Google          ProviderConfig
	Apple           ProviderConfig
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	LoginCodeTTL    time.Duration
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:      envDefault("SSHKING_LISTEN_ADDR", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("SSHKING_DATABASE_URL")),
		PublicURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("SSHKING_PUBLIC_URL")), "/"),
		Google:          ProviderConfig{ClientID: strings.TrimSpace(os.Getenv("SSHKING_GOOGLE_CLIENT_ID")), ClientSecret: strings.TrimSpace(os.Getenv("SSHKING_GOOGLE_CLIENT_SECRET"))},
		Apple:           ProviderConfig{ClientID: strings.TrimSpace(os.Getenv("SSHKING_APPLE_CLIENT_ID")), ClientSecret: strings.TrimSpace(os.Getenv("SSHKING_APPLE_CLIENT_SECRET"))},
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		LoginCodeTTL:    5 * time.Minute,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("SSHKING_DATABASE_URL is required")
	}
	return cfg, nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
