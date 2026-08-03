package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"coreloop/backend/internal/ids"
)

type DeliveryPart struct {
	ID           string
	LessonPartID string
	Sequence     int
	Text         string
	State        string
}
type DeliveryBundle struct {
	ID           string
	UserID       string
	AssignmentID string
	ChatID       string
	State        string
	Parts        []DeliveryPart
}

func (store *Store) PrepareDelivery(ctx context.Context, userID, assignmentID, jobID string, now time.Time) (DeliveryBundle, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryBundle{}, err
	}
	defer tx.Rollback()
	var destinationID, chatID string
	if err := tx.QueryRowContext(ctx, `SELECT id,telegram_chat_id FROM delivery_destinations WHERE user_id=? AND enabled=1 AND status='connected'`, userID).Scan(&destinationID, &chatID); err != nil {
		return DeliveryBundle{}, err
	}
	idempotency := "delivery:" + assignmentID
	deliveryID := mustID("del")
	_, err = tx.ExecContext(ctx, `INSERT INTO deliveries
		(id,user_id,assignment_id,destination_id,job_id,intended_at,idempotency_key)
		VALUES (?,?,?,?,?,?,?) ON CONFLICT(idempotency_key) DO NOTHING`, deliveryID, userID, assignmentID, destinationID, jobID, timestamp(now), idempotency)
	if err != nil {
		return DeliveryBundle{}, err
	}
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT id,bundle_state FROM deliveries WHERE idempotency_key=?`, idempotency).Scan(&deliveryID, &state); err != nil {
		return DeliveryBundle{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT lp.id,lp.sequence_number,lp.rendered_text FROM lesson_assignments la
		JOIN lesson_parts lp ON lp.lesson_id=la.lesson_id WHERE la.id=? AND la.user_id=? ORDER BY lp.sequence_number`, assignmentID, userID)
	if err != nil {
		return DeliveryBundle{}, err
	}
	var raw []DeliveryPart
	for rows.Next() {
		var part DeliveryPart
		if err := rows.Scan(&part.LessonPartID, &part.Sequence, &part.Text); err != nil {
			rows.Close()
			return DeliveryBundle{}, err
		}
		raw = append(raw, part)
	}
	rows.Close()
	for index := range raw {
		partID := mustID("dpt")
		_, err = tx.ExecContext(ctx, `INSERT INTO delivery_parts
		(id,user_id,delivery_id,lesson_part_id,sequence_number,idempotency_key) VALUES (?,?,?,?,?,?) ON CONFLICT(idempotency_key) DO NOTHING`, partID, userID, deliveryID, raw[index].LessonPartID, raw[index].Sequence, idempotency+":"+fmt.Sprint(raw[index].Sequence))
		if err != nil {
			return DeliveryBundle{}, err
		}
	}
	partRows, err := tx.QueryContext(ctx, `SELECT dp.id,dp.lesson_part_id,dp.sequence_number,lp.rendered_text,dp.state FROM delivery_parts dp JOIN lesson_parts lp ON lp.id=dp.lesson_part_id WHERE dp.delivery_id=? ORDER BY dp.sequence_number`, deliveryID)
	if err != nil {
		return DeliveryBundle{}, err
	}
	var parts []DeliveryPart
	for partRows.Next() {
		var part DeliveryPart
		if err := partRows.Scan(&part.ID, &part.LessonPartID, &part.Sequence, &part.Text, &part.State); err != nil {
			partRows.Close()
			return DeliveryBundle{}, err
		}
		parts = append(parts, part)
	}
	partRows.Close()
	if _, err := tx.ExecContext(ctx, `UPDATE deliveries SET bundle_state='sending',started_at=COALESCE(started_at,?),attempt_count=attempt_count+1,updated_at=? WHERE id=? AND bundle_state<>'delivered'`, timestamp(now), timestamp(now), deliveryID); err != nil {
		return DeliveryBundle{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeliveryBundle{}, err
	}
	return DeliveryBundle{ID: deliveryID, UserID: userID, AssignmentID: assignmentID, ChatID: chatID, State: state, Parts: parts}, nil
}

func (store *Store) CompleteDeliveryPart(ctx context.Context, deliveryPartID, messageID string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE delivery_parts SET state='delivered',telegram_message_id=?,sent_at=?,confirmed_at=?,updated_at=? WHERE id=?`, messageID, timestamp(now), timestamp(now), timestamp(now), deliveryPartID)
	return err
}

func (store *Store) CompleteDelivery(ctx context.Context, deliveryID, assignmentID string, now time.Time) error {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM delivery_parts WHERE delivery_id=? AND state<>'delivered'`, deliveryID).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deliveries SET bundle_state='delivered',completed_at=?,updated_at=? WHERE id=?`, timestamp(now), timestamp(now), deliveryID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE lesson_assignments SET state='delivered',delivered_at=COALESCE(delivered_at,?) WHERE id=? AND state='queued'`, timestamp(now), assignmentID); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *Store) FailDelivery(ctx context.Context, deliveryID, code string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE deliveries SET bundle_state='partial',last_error_code=?,last_error_at=?,updated_at=? WHERE id=?`, code, timestamp(now), timestamp(now), deliveryID)
	return err
}

func (store *Store) NotificationOnce(ctx context.Context, userID, kind, key string) (string, bool, error) {
	id, err := ids.New("ntf")
	if err != nil {
		return "", false, err
	}
	var user any
	if userID != "" {
		user = userID
	}
	result, err := store.database.ExecContext(ctx, `INSERT INTO notification_events (id,user_id,kind,deduplication_key) VALUES (?,?,?,?) ON CONFLICT(deduplication_key) DO NOTHING`, id, user, kind, key)
	if err != nil {
		return "", false, err
	}
	changed, _ := result.RowsAffected()
	return id, changed == 1, nil
}

func (store *Store) CompleteNotification(ctx context.Context, id string, now time.Time, errorCode string) error {
	var delivered any
	if errorCode == "" {
		delivered = timestamp(now)
	}
	_, err := store.database.ExecContext(ctx, `UPDATE notification_events SET delivered_at=?,last_error_code=? WHERE id=?`, delivered, errorCode, id)
	return err
}
