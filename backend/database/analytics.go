package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type AnalyticsConnection struct {
	ID                    int64      `json:"id"`
	UserID                int64      `json:"user_id"`
	Provider              string     `json:"provider"`
	EncryptedAccessToken  string     `json:"-"`
	EncryptedRefreshToken string     `json:"-"`
	ExpiresAt             *time.Time `json:"expires_at"`
	CreatedAt             time.Time  `json:"created_at"`
}

// UpsertAnalyticsConnection stores (or replaces) a user's provider connection.
func (s *Store) UpsertAnalyticsConnection(ctx context.Context, userID int64, provider,
	encAccess, encRefresh string, expiresAt *time.Time) (AnalyticsConnection, error) {
	var c AnalyticsConnection
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO analytics_connections
		 (user_id, provider, encrypted_access_token, encrypted_refresh_token, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, provider) DO UPDATE
		 SET encrypted_access_token = EXCLUDED.encrypted_access_token,
		     encrypted_refresh_token = EXCLUDED.encrypted_refresh_token,
		     expires_at = EXCLUDED.expires_at,
		     updated_at = now()
		 RETURNING id, user_id, provider, encrypted_access_token, encrypted_refresh_token, expires_at, created_at`,
		userID, provider, encAccess, encRefresh, expiresAt).
		Scan(&c.ID, &c.UserID, &c.Provider, &c.EncryptedAccessToken, &c.EncryptedRefreshToken, &c.ExpiresAt, &c.CreatedAt)
	return c, err
}

// GetAnalyticsConnection fetches one user+provider connection.
func (s *Store) GetAnalyticsConnection(ctx context.Context, userID int64, provider string) (AnalyticsConnection, error) {
	var c AnalyticsConnection
	err := s.Pool.QueryRow(ctx,
		`SELECT id, user_id, provider, encrypted_access_token, encrypted_refresh_token, expires_at, created_at
		 FROM analytics_connections WHERE user_id = $1 AND provider = $2`,
		userID, provider).
		Scan(&c.ID, &c.UserID, &c.Provider, &c.EncryptedAccessToken, &c.EncryptedRefreshToken, &c.ExpiresAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AnalyticsConnection{}, ErrNotFound
	}
	return c, err
}

// ListAnalyticsConnections lists a user's connections (for the UI).
func (s *Store) ListAnalyticsConnections(ctx context.Context, userID int64) ([]AnalyticsConnection, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, user_id, provider, encrypted_access_token, encrypted_refresh_token, expires_at, created_at
		 FROM analytics_connections WHERE user_id = $1 ORDER BY provider`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AnalyticsConnection{}
	for rows.Next() {
		var c AnalyticsConnection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Provider, &c.EncryptedAccessToken,
			&c.EncryptedRefreshToken, &c.ExpiresAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteAnalyticsConnection removes a connection (ownership enforced).
func (s *Store) DeleteAnalyticsConnection(ctx context.Context, userID int64, provider string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM analytics_connections WHERE user_id = $1 AND provider = $2`, userID, provider)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
