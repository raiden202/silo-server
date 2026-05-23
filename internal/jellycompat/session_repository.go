package jellycompat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const jellycompatSessionColumns = `token, username, account_username, profile_id, profile_name, pseudo_user_id, streamapp_user_id, streamapp_access_token, streamapp_refresh_token, streamapp_token_expiry, created_at, expires_at`

// SessionRepository persists compat sessions in PostgreSQL.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository creates a new compat session repository.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func scanCompatSession(row pgx.Row) (*Session, error) {
	var session Session
	err := row.Scan(
		&session.Token,
		&session.Username,
		&session.AccountUsername,
		&session.ProfileID,
		&session.ProfileName,
		&session.PseudoUserID,
		&session.StreamAppUserID,
		&session.StreamAppAccessToken,
		&session.StreamAppRefreshToken,
		&session.StreamAppTokenExpiry,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("scan compat session: %w", err)
	}
	return &session, nil
}

// Upsert inserts or updates a compat session.
func (r *SessionRepository) Upsert(ctx context.Context, session Session) error {
	if session.Token == "" {
		session.Token = uuid.NewString()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO jellycompat_sessions (
			token, username, account_username, profile_id, profile_name, pseudo_user_id,
			streamapp_user_id, streamapp_access_token, streamapp_refresh_token,
			streamapp_token_expiry, created_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12
		)
		ON CONFLICT (token) DO UPDATE SET
			username = EXCLUDED.username,
			account_username = EXCLUDED.account_username,
			profile_id = EXCLUDED.profile_id,
			profile_name = EXCLUDED.profile_name,
			pseudo_user_id = EXCLUDED.pseudo_user_id,
			streamapp_user_id = EXCLUDED.streamapp_user_id,
			streamapp_access_token = EXCLUDED.streamapp_access_token,
			streamapp_refresh_token = EXCLUDED.streamapp_refresh_token,
			streamapp_token_expiry = EXCLUDED.streamapp_token_expiry,
			created_at = EXCLUDED.created_at,
			expires_at = EXCLUDED.expires_at
	`,
		session.Token,
		session.Username,
		session.AccountUsername,
		session.ProfileID,
		session.ProfileName,
		session.PseudoUserID,
		session.StreamAppUserID,
		session.StreamAppAccessToken,
		session.StreamAppRefreshToken,
		session.StreamAppTokenExpiry,
		session.CreatedAt,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("upsert compat session: %w", err)
	}
	return nil
}

// GetByToken loads an active compat session by token.
func (r *SessionRepository) GetByToken(ctx context.Context, token string, now time.Time) (*Session, error) {
	return scanCompatSession(r.pool.QueryRow(ctx,
		`SELECT `+jellycompatSessionColumns+`
		FROM jellycompat_sessions
		WHERE token = $1 AND expires_at > $2`,
		token, now,
	))
}

// DeleteByToken removes a compat session by token.
func (r *SessionRepository) DeleteByToken(ctx context.Context, token string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM jellycompat_sessions WHERE token = $1`, token)
	if err != nil {
		return fmt.Errorf("delete compat session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// DeleteExpired removes expired compat sessions.
func (r *SessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM jellycompat_sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired compat sessions: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// DeleteByUserID removes all compat sessions for a given Silo user.
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM jellycompat_sessions WHERE streamapp_user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("delete compat sessions by user: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
