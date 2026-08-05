package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/radar"
)

// EnqueueManualRadar atomically reserves the best currently eligible Radar
// candidate and creates its durable delivery job. It deliberately does not
// update radar_daily_usage or released_at, so an owner acceptance test cannot
// consume or delay normal Radar delivery.
func (store *Store) EnqueueManualRadar(
	ctx context.Context,
	userID string,
	idempotencyKey string,
	now time.Time,
) (string, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var existingJobID string
	err = tx.QueryRowContext(
		ctx,
		"SELECT id FROM job_queue WHERE idempotency_key=?",
		idempotencyKey,
	).Scan(&existingJobID)
	if err == nil {
		return existingJobID, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var candidateID string
	rows, err := tx.QueryContext(ctx, `SELECT rc.id,si.normalized_url
		FROM radar_candidates rc
		JOIN source_items si ON si.id=rc.source_item_id
		JOIN users u ON u.id=rc.user_id AND u.status='active'
		JOIN learning_preferences lp ON lp.user_id=rc.user_id AND lp.radar_enabled=1
		JOIN delivery_destinations dd ON dd.user_id=rc.user_id
			AND dd.channel='telegram' AND dd.enabled=1 AND dd.status='connected'
		WHERE rc.user_id=? AND rc.status='pending'
			AND rc.ranker_version=? AND rc.relevance_score>=?
			AND COALESCE(si.published_at,si.retrieved_at)>=?
		ORDER BY rc.relevance_score DESC,
			COALESCE(si.published_at,si.retrieved_at) DESC,rc.created_at,rc.id
		LIMIT 20`, userID, radar.RankerVersion, radar.MinimumDeliveryScore,
		timestamp(now.Add(-radar.MaximumItemAge)))
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var currentID, currentURL string
		if err := rows.Scan(&currentID, &currentURL); err != nil {
			rows.Close()
			return "", err
		}
		if _, err := radar.CanonicalURL(currentURL); err == nil {
			candidateID = currentID
			break
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	if candidateID == "" {
		return "", sql.ErrNoRows
	}

	result, err := tx.ExecContext(ctx, `UPDATE radar_candidates
		SET status='qualified',updated_at=?
		WHERE id=? AND user_id=? AND status='pending'`,
		timestamp(now), candidateID, userID)
	if err != nil {
		return "", err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if changed != 1 {
		return "", fmt.Errorf("reserve manual Radar candidate: %w", sql.ErrNoRows)
	}

	jobID, err := ids.New("job")
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{"candidate_id": candidateID})
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO job_queue
		(id,user_id,job_type,due_at,idempotency_key,payload_json)
		VALUES (?,?,?,?,?,?)`, jobID, userID, "deliver_radar", timestamp(now),
		idempotencyKey, string(payload))
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return jobID, nil
}

func (store *Store) RadarCandidateDeliveryState(
	ctx context.Context,
	userID string,
	candidateID string,
) (string, string, error) {
	var candidateState, deliveryState string
	err := store.database.QueryRowContext(ctx, `SELECT rc.status,COALESCE((
		SELECT rd.state FROM radar_deliveries rd
		WHERE rd.candidate_id=rc.id AND rd.user_id=rc.user_id
		ORDER BY rd.created_at DESC LIMIT 1
	),'') FROM radar_candidates rc WHERE rc.id=? AND rc.user_id=?`,
		candidateID, userID).Scan(&candidateState, &deliveryState)
	return candidateState, deliveryState, err
}
