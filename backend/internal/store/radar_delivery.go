package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"coreloop/backend/internal/ids"
)

type RadarDeliveryPart struct {
	ID, Text, State string
	Sequence        int
}

type RadarDeliveryBundle struct {
	ID, CandidateID, UserID, ChatID, State string
	Parts                                  []RadarDeliveryPart
}

func (store *Store) PrepareRadarDelivery(
	ctx context.Context,
	userID string,
	candidateID string,
	jobID string,
	parts []string,
	now time.Time,
) (RadarDeliveryBundle, error) {
	if len(parts) == 0 {
		return RadarDeliveryBundle{}, fmt.Errorf("Radar delivery has no parts: %w", sql.ErrNoRows)
	}
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return RadarDeliveryBundle{}, err
	}
	defer tx.Rollback()
	var destinationID, chatID string
	if err := tx.QueryRowContext(ctx, `SELECT id,telegram_chat_id FROM delivery_destinations
		WHERE user_id=? AND enabled=1 AND status='connected'`, userID).
		Scan(&destinationID, &chatID); err != nil {
		return RadarDeliveryBundle{}, err
	}
	deliveryID, err := ids.New("rdel")
	if err != nil {
		return RadarDeliveryBundle{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO radar_deliveries
		(id,user_id,candidate_id,destination_id,job_id) VALUES (?,?,?,?,?)
		ON CONFLICT(candidate_id) DO NOTHING`,
		deliveryID, userID, candidateID, destinationID, jobID)
	if err != nil {
		return RadarDeliveryBundle{}, err
	}
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT id,state FROM radar_deliveries
		WHERE candidate_id=? AND user_id=?`, candidateID, userID).
		Scan(&deliveryID, &state); err != nil {
		return RadarDeliveryBundle{}, err
	}
	var persistedParts int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM radar_delivery_parts
		WHERE delivery_id=?`, deliveryID).Scan(&persistedParts); err != nil {
		return RadarDeliveryBundle{}, err
	}
	// The first attempt freezes the rendered bundle. A later retry can happen
	// after provider availability changes, but must never mix new text with
	// parts that Telegram already accepted.
	if persistedParts == 0 {
		values := strings.TrimSuffix(strings.Repeat("(?,?,?,?,?),", len(parts)), ",")
		arguments := make([]any, 0, len(parts)*5)
		for index, part := range parts {
			partID, err := ids.New("rdpt")
			if err != nil {
				return RadarDeliveryBundle{}, err
			}
			arguments = append(arguments, partID, userID, deliveryID, index+1, part)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO radar_delivery_parts
			(id,user_id,delivery_id,sequence_number,rendered_text) VALUES `+values,
			arguments...,
		); err != nil {
			return RadarDeliveryBundle{}, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,sequence_number,rendered_text,state
		FROM radar_delivery_parts WHERE delivery_id=? ORDER BY sequence_number`, deliveryID)
	if err != nil {
		return RadarDeliveryBundle{}, err
	}
	var deliveryParts []RadarDeliveryPart
	for rows.Next() {
		var part RadarDeliveryPart
		if err := rows.Scan(&part.ID, &part.Sequence, &part.Text, &part.State); err != nil {
			rows.Close()
			return RadarDeliveryBundle{}, err
		}
		deliveryParts = append(deliveryParts, part)
	}
	if err := rows.Close(); err != nil {
		return RadarDeliveryBundle{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE radar_deliveries SET state='sending',
		started_at=COALESCE(started_at,?),attempt_count=attempt_count+1,updated_at=?
		WHERE id=? AND state<>'delivered'`, timestamp(now), timestamp(now), deliveryID); err != nil {
		return RadarDeliveryBundle{}, err
	}
	if err := tx.Commit(); err != nil {
		return RadarDeliveryBundle{}, err
	}
	return RadarDeliveryBundle{
		ID: deliveryID, CandidateID: candidateID, UserID: userID,
		ChatID: chatID, State: state, Parts: deliveryParts,
	}, nil
}

func (store *Store) CompleteRadarDeliveryPart(
	ctx context.Context,
	partID string,
	messageID string,
	now time.Time,
) error {
	_, err := store.database.ExecContext(ctx, `UPDATE radar_delivery_parts SET state='delivered',
		telegram_message_id=?,sent_at=?,last_error_code=NULL,updated_at=? WHERE id=?`,
		messageID, timestamp(now), timestamp(now), partID)
	return err
}

func (store *Store) CompleteRadarDelivery(
	ctx context.Context,
	deliveryID string,
	userID string,
	candidateID string,
	now time.Time,
) error {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM radar_delivery_parts
		WHERE delivery_id=? AND state<>'delivered'`, deliveryID).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE radar_deliveries SET state='delivered',
		completed_at=?,last_error_code=NULL,updated_at=? WHERE id=?`,
		timestamp(now), timestamp(now), deliveryID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE radar_candidates SET status='delivered',updated_at=?
		WHERE id=? AND user_id=? AND status IN ('qualified','delivered')`,
		timestamp(now), candidateID, userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (store *Store) FailRadarDelivery(ctx context.Context, deliveryID, code string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE radar_deliveries SET state='partial',
		last_error_code=?,last_error_at=?,updated_at=? WHERE id=?`,
		code, timestamp(now), timestamp(now), deliveryID)
	return err
}
