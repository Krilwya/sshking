package cloudapi

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type store struct{ pool *pgxpool.Pool }

type authFlow struct {
	Provider, RedirectURI, CodeChallenge string
}

type user struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	AvatarURL   string `json:"avatarUrl"`
}

func newStore(ctx context.Context, databaseURL string) (*store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &store{pool: pool}, nil
}

func (s *store) close()                         { s.pool.Close() }
func (s *store) ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *store) createFlow(ctx context.Context, stateHash []byte, provider, redirectURI, challenge string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO auth_flows(state_hash, provider, redirect_uri, code_challenge, expires_at) VALUES($1,$2,$3,$4,$5)`, stateHash, provider, redirectURI, challenge, expires)
	return err
}

func (s *store) consumeFlow(ctx context.Context, stateHash []byte) (authFlow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return authFlow{}, err
	}
	defer tx.Rollback(ctx)
	var f authFlow
	err = tx.QueryRow(ctx, `DELETE FROM auth_flows WHERE state_hash=$1 AND expires_at>now() RETURNING provider, redirect_uri, code_challenge`, stateHash).Scan(&f.Provider, &f.RedirectURI, &f.CodeChallenge)
	if err != nil {
		return authFlow{}, err
	}
	return f, tx.Commit(ctx)
}

func (s *store) upsertIdentity(ctx context.Context, provider, subject, email, name, avatar string) (user, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return user{}, err
	}
	defer tx.Rollback(ctx)
	var u user
	err = tx.QueryRow(ctx, `SELECT u.id::text,u.display_name,u.email,u.avatar_url FROM identities i JOIN users u ON u.id=i.user_id WHERE i.provider=$1 AND i.subject=$2`, provider, subject).Scan(&u.ID, &u.DisplayName, &u.Email, &u.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		u.ID = uuid.NewString()
		u.DisplayName = name
		u.Email = email
		u.AvatarURL = avatar
		_, err = tx.Exec(ctx, `INSERT INTO users(id,display_name,email,avatar_url) VALUES($1,$2,$3,$4)`, u.ID, name, email, avatar)
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO identities(provider,subject,user_id,email) VALUES($1,$2,$3,$4)`, provider, subject, u.ID, email)
		}
	} else if err == nil {
		u.DisplayName = name
		u.Email = email
		u.AvatarURL = avatar
		_, err = tx.Exec(ctx, `UPDATE users SET display_name=$2,email=$3,avatar_url=$4,updated_at=now() WHERE id=$1`, u.ID, name, email, avatar)
	}
	if err != nil {
		return user{}, err
	}
	return u, tx.Commit(ctx)
}

func (s *store) createLoginCode(ctx context.Context, hash []byte, userID, challenge, state string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO login_codes(code_hash,user_id,code_challenge,state,expires_at) VALUES($1,$2,$3,$4,$5)`, hash, userID, challenge, state, expires)
	return err
}

func (s *store) consumeLoginCode(ctx context.Context, hash []byte, challenge string) (userID string, err error) {
	err = s.pool.QueryRow(ctx, `DELETE FROM login_codes WHERE code_hash=$1 AND code_challenge=$2 AND expires_at>now() RETURNING user_id::text`, hash, challenge).Scan(&userID)
	return
}

func (s *store) createSession(ctx context.Context, userID, deviceName string, accessHash, refreshHash []byte, accessExpiry, refreshExpiry time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO device_sessions(id,user_id,device_name,access_hash,refresh_hash,access_expires_at,refresh_expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, uuid.NewString(), userID, deviceName, accessHash, refreshHash, accessExpiry, refreshExpiry)
	return err
}

func (s *store) userByAccess(ctx context.Context, hash []byte) (user, error) {
	var u user
	err := s.pool.QueryRow(ctx, `SELECT u.id::text,u.display_name,u.email,u.avatar_url FROM device_sessions d JOIN users u ON u.id=d.user_id WHERE d.access_hash=$1 AND d.access_expires_at>now() AND d.revoked_at IS NULL`, hash).Scan(&u.ID, &u.DisplayName, &u.Email, &u.AvatarURL)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `UPDATE device_sessions SET last_seen_at=now() WHERE access_hash=$1`, hash)
	}
	return u, err
}

func (s *store) rotateSession(ctx context.Context, oldHash, accessHash, refreshHash []byte, accessExpiry, refreshExpiry time.Time) (user, error) {
	var u user
	err := s.pool.QueryRow(ctx, `UPDATE device_sessions d SET access_hash=$2,refresh_hash=$3,access_expires_at=$4,refresh_expires_at=$5,last_seen_at=now() FROM users u WHERE d.refresh_hash=$1 AND d.refresh_expires_at>now() AND d.revoked_at IS NULL AND u.id=d.user_id RETURNING u.id::text,u.display_name,u.email,u.avatar_url`, oldHash, accessHash, refreshHash, accessExpiry, refreshExpiry).Scan(&u.ID, &u.DisplayName, &u.Email, &u.AvatarURL)
	return u, err
}
