package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"coreloop/backend/internal/ids"
)

func (store *Store) CreateInvite(ctx context.Context, tokenHash, creatorID string, expiresAt time.Time) (Invite, error) {
	id, err := ids.New("inv")
	if err != nil {
		return Invite{}, err
	}
	var creator any
	if creatorID != "" {
		creator = creatorID
	}
	_, err = store.database.ExecContext(ctx, `INSERT INTO invites
		(id, token_hash, created_by_user_id, expires_at) VALUES (?, ?, ?, ?)`, id, tokenHash, creator, timestamp(expiresAt))
	if err != nil {
		return Invite{}, fmt.Errorf("create invite: %w", err)
	}
	return Invite{ID: id, ExpiresAt: expiresAt}, nil
}

func (store *Store) ResolveInvite(ctx context.Context, tokenHash string, now time.Time) (Invite, error) {
	var invite Invite
	var expires string
	var consumed sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT id, expires_at, consumed_at FROM invites
		WHERE token_hash = ?`, tokenHash).Scan(&invite.ID, &expires, &consumed)
	if err != nil {
		return Invite{}, err
	}
	invite.ExpiresAt, err = parseTimestamp(expires)
	if err != nil {
		return Invite{}, err
	}
	invite.Consumed = consumed.Valid
	if invite.Consumed || !invite.ExpiresAt.After(now) {
		return Invite{}, sql.ErrNoRows
	}
	return invite, nil
}

func (store *Store) CreateAuthFlow(ctx context.Context, flow AuthFlow, stateHash string) error {
	var invite any
	if flow.InviteID != "" {
		invite = flow.InviteID
	}
	_, err := store.database.ExecContext(ctx, `INSERT INTO auth_flows
		(id, state_hash, invite_id, code_verifier, nonce, return_path, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, flow.ID, stateHash, invite, flow.CodeVerifier, flow.Nonce,
		flow.ReturnPath, timestamp(flow.ExpiresAt))
	return err
}

func (store *Store) AllowRateLimit(
	ctx context.Context,
	bucketKey string,
	now time.Time,
	window time.Duration,
	limit int,
) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	var requestCount int
	err := store.database.QueryRowContext(ctx, `INSERT INTO rate_limits
		(bucket_key,window_started_at,request_count,updated_at) VALUES (?,?,1,?)
		ON CONFLICT(bucket_key) DO UPDATE SET
			window_started_at=CASE WHEN rate_limits.window_started_at<=? THEN excluded.window_started_at ELSE rate_limits.window_started_at END,
			request_count=CASE WHEN rate_limits.window_started_at<=? THEN 1 ELSE rate_limits.request_count+1 END,
			updated_at=excluded.updated_at
		WHERE rate_limits.window_started_at<=? OR rate_limits.request_count<?
		RETURNING request_count`,
		bucketKey, timestamp(now), timestamp(now), timestamp(now.Add(-window)),
		timestamp(now.Add(-window)), timestamp(now.Add(-window)), limit,
	).Scan(&requestCount)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && requestCount <= limit, err
}

func (store *Store) ConsumeAuthFlow(ctx context.Context, stateHash string, now time.Time) (AuthFlow, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return AuthFlow{}, err
	}
	defer tx.Rollback()
	var flow AuthFlow
	var invite sql.NullString
	var expires string
	err = tx.QueryRowContext(ctx, `SELECT id, invite_id, code_verifier, nonce, return_path, expires_at
		FROM auth_flows WHERE state_hash = ? AND used_at IS NULL AND expires_at > ?`, stateHash, timestamp(now)).
		Scan(&flow.ID, &invite, &flow.CodeVerifier, &flow.Nonce, &flow.ReturnPath, &expires)
	if err != nil {
		return AuthFlow{}, err
	}
	flow.InviteID = invite.String
	flow.ExpiresAt, err = parseTimestamp(expires)
	if err != nil {
		return AuthFlow{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE auth_flows SET used_at = ? WHERE id = ? AND used_at IS NULL`, timestamp(now), flow.ID)
	if err != nil {
		return AuthFlow{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return AuthFlow{}, sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return AuthFlow{}, err
	}
	return flow, nil
}

type Identity struct {
	Subject        string
	TelegramChatID string
	DisplayName    string
	Username       string
	AvatarURL      string
}

func (store *Store) UpsertUserFromTelegram(ctx context.Context, identity Identity, inviteID string, now time.Time) (User, bool, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return User{}, false, err
	}
	defer tx.Rollback()
	user, err := findUserBySubject(ctx, tx, identity.Subject)
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		if inviteID == "" {
			return User{}, false, sql.ErrNoRows
		}
		id, idErr := ids.New("usr")
		if idErr != nil {
			return User{}, false, idErr
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO users
			(id, telegram_subject, display_name, username, avatar_url) VALUES (?, ?, ?, ?, ?)`,
			id, identity.Subject, identity.DisplayName, identity.Username, identity.AvatarURL)
		if err != nil {
			return User{}, false, err
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE invites SET consumed_at = ?, consumed_by_user_id = ?
			WHERE id = ? AND consumed_at IS NULL AND expires_at > ?`, timestamp(now), id, inviteID, timestamp(now))
		if updateErr != nil {
			return User{}, false, updateErr
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return User{}, false, sql.ErrNoRows
		}
		user = User{ID: id, TelegramSubject: identity.Subject, DisplayName: identity.DisplayName,
			Username: identity.Username, AvatarURL: identity.AvatarURL, Status: "active", CreatedAt: now}
		if err := createUserDefaults(ctx, tx, id); err != nil {
			return User{}, false, err
		}
		created = true
	} else if err != nil {
		return User{}, false, err
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE users SET display_name = ?, username = ?, avatar_url = ?,
			updated_at = ? WHERE id = ?`, identity.DisplayName, identity.Username, identity.AvatarURL, timestamp(now), user.ID)
		if err != nil {
			return User{}, false, err
		}
		user.DisplayName, user.Username, user.AvatarURL = identity.DisplayName, identity.Username, identity.AvatarURL
	}
	destinationID, err := ids.New("dst")
	if err != nil {
		return User{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO delivery_destinations
		(id, user_id, telegram_chat_id) VALUES (?, ?, ?)
		ON CONFLICT(user_id, channel) DO UPDATE SET telegram_chat_id=excluded.telegram_chat_id,
		status='connected', enabled=1, connected_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`, destinationID, user.ID, identity.TelegramChatID); err != nil {
		return User{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, false, err
	}
	return user, created, nil
}

func createUserDefaults(ctx context.Context, tx *sql.Tx, userID string) error {
	profileID, err := ids.New("prf")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO learning_profiles (id, user_id) VALUES (?, ?)", profileID, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO learning_preferences (user_id) VALUES (?)", userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_topic_preferences (user_id,topic_id)
		SELECT ?,id FROM topics WHERE status='active'`, userID); err != nil {
		return err
	}
	return insertDeliverySchedules(
		ctx, tx, userID, []string{"08:30", "13:00", "20:30"},
		"Asia/Kolkata", false,
	)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func findUserBySubject(ctx context.Context, query queryRower, subject string) (User, error) {
	var user User
	var created string
	err := query.QueryRowContext(ctx, `SELECT id, telegram_subject, display_name, username, avatar_url, status, created_at
		FROM users WHERE telegram_subject = ? AND status <> 'deleted'`, subject).
		Scan(&user.ID, &user.TelegramSubject, &user.DisplayName, &user.Username, &user.AvatarURL, &user.Status, &created)
	if err != nil {
		return User{}, err
	}
	user.CreatedAt, err = parseTimestamp(created)
	return user, err
}

func (store *Store) CreateSession(ctx context.Context, userID, tokenHash, csrfHash string, expiresAt time.Time) (string, error) {
	id, err := ids.New("ses")
	if err != nil {
		return "", err
	}
	_, err = store.database.ExecContext(ctx, `INSERT INTO sessions
		(id, user_id, token_hash, csrf_hash, expires_at) VALUES (?, ?, ?, ?, ?)`, id, userID, tokenHash, csrfHash, timestamp(expiresAt))
	return id, err
}

func (store *Store) SessionByToken(ctx context.Context, tokenHash string, now time.Time) (Session, User, error) {
	var session Session
	var user User
	var expires, created string
	var revoked sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT s.id, s.user_id, s.csrf_hash, s.expires_at, s.revoked_at,
		u.telegram_subject, u.display_name, u.username, u.avatar_url, u.status, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.status = 'active'`,
		tokenHash, timestamp(now)).Scan(&session.ID, &session.UserID, &session.CSRFHash, &expires, &revoked,
		&user.TelegramSubject, &user.DisplayName, &user.Username, &user.AvatarURL, &user.Status, &created)
	if err != nil {
		return Session{}, User{}, err
	}
	user.ID = session.UserID
	session.ExpiresAt, err = parseTimestamp(expires)
	if err != nil {
		return Session{}, User{}, err
	}
	user.CreatedAt, err = parseTimestamp(created)
	if err != nil {
		return Session{}, User{}, err
	}
	return session, user, nil
}

func (store *Store) RevokeSession(ctx context.Context, sessionID string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, "UPDATE sessions SET revoked_at = ? WHERE id = ?", timestamp(now), sessionID)
	return err
}

func (store *Store) RevokeAllSessions(ctx context.Context, userID string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, timestamp(now), userID)
	return err
}

func (store *Store) Destination(ctx context.Context, userID string) (string, error) {
	var chatID string
	err := store.database.QueryRowContext(ctx, `SELECT telegram_chat_id FROM delivery_destinations
		WHERE user_id = ? AND channel='telegram' AND enabled=1 AND status='connected'`, userID).Scan(&chatID)
	return chatID, err
}

func (store *Store) DisconnectDestination(ctx context.Context, userID string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE delivery_destinations
		SET status='disconnected',enabled=0,updated_at=?
		WHERE user_id=? AND channel='telegram'`, timestamp(now), userID)
	return err
}

func (store *Store) UserByTelegramChatID(ctx context.Context, chatID string) (User, error) {
	var user User
	var created string
	err := store.database.QueryRowContext(ctx, `SELECT u.id,u.telegram_subject,u.display_name,
		u.username,u.avatar_url,u.status,u.created_at
		FROM users u JOIN delivery_destinations dd ON dd.user_id=u.id
		WHERE dd.channel='telegram' AND dd.telegram_chat_id=? AND dd.enabled=1
			AND dd.status='connected' AND u.status<>'deleted'`, chatID).
		Scan(&user.ID, &user.TelegramSubject, &user.DisplayName, &user.Username,
			&user.AvatarURL, &user.Status, &created)
	if err != nil {
		return User{}, err
	}
	user.CreatedAt, err = parseTimestamp(created)
	return user, err
}

func (store *Store) DeleteUser(ctx context.Context, userID string, now time.Time) error {
	_ = now
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id=?", userID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02T15:04:05.000Z", value)
}

func encodeStrings(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}
