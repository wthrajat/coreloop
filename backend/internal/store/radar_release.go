package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"coreloop/backend/internal/radar"
)

const (
	maximumRadarReleasesPerPass = 50
	maximumUnlimitedReleasePass = 20
	radarCandidateMaxAge        = radar.MaximumItemAge
)

type radarReleaseUser struct {
	ID              string
	ItemsPerDay     int
	WeekendsEnabled bool
	TimeZone        string
}

type radarReleaseCandidate struct {
	ID, SourceID string
	Score        float64
}

// ReleaseRadarCandidates atomically reserves per-user daily slots and creates
// durable delivery jobs. Candidate selection and job creation share a
// transaction so a process failure cannot leave a qualified item without a
// wakeable job.
func (store *Store) ReleaseRadarCandidates(ctx context.Context, now time.Time) (int, error) {
	if _, err := store.database.ExecContext(ctx, `UPDATE radar_candidates
		SET status='rejected',rejection_reason='superseded_ranking_policy',updated_at=?
		WHERE status='pending' AND ranker_version<>?`,
		timestamp(now), radar.RankerVersion); err != nil {
		return 0, err
	}
	if _, err := store.database.ExecContext(ctx, `UPDATE radar_candidates
		SET status='rejected',rejection_reason='below_editorial_threshold',updated_at=?
		WHERE status='pending' AND ranker_version=? AND relevance_score<?`,
		timestamp(now), radar.RankerVersion, radar.MinimumDeliveryScore); err != nil {
		return 0, err
	}
	if _, err := store.database.ExecContext(ctx, `UPDATE radar_candidates
		SET status='rejected',rejection_reason='expired_before_release',updated_at=?
		WHERE status='pending' AND source_item_id IN (
			SELECT id FROM source_items WHERE COALESCE(published_at,retrieved_at)<?
		)`, timestamp(now), timestamp(now.Add(-radarCandidateMaxAge))); err != nil {
		return 0, err
	}
	users, err := store.radarReleaseUsers(ctx)
	if err != nil {
		return 0, err
	}
	released := 0
	for _, user := range users {
		if released >= maximumRadarReleasesPerPass {
			break
		}
		count, releaseErr := store.releaseRadarForUser(
			ctx, user, now, maximumRadarReleasesPerPass-released,
		)
		if releaseErr != nil {
			return released, releaseErr
		}
		released += count
	}
	return released, nil
}

func (store *Store) radarReleaseUsers(ctx context.Context) ([]radarReleaseUser, error) {
	rows, err := store.database.QueryContext(ctx, `SELECT lp.user_id,lp.radar_items_per_day,
		lp.radar_weekends_enabled,lp.time_zone
		FROM learning_preferences lp
		JOIN users u ON u.id=lp.user_id AND u.status='active'
		JOIN delivery_destinations dd ON dd.user_id=lp.user_id AND dd.channel='telegram'
			AND dd.enabled=1 AND dd.status='connected'
		WHERE lp.radar_enabled=1 AND EXISTS (
			SELECT 1 FROM radar_candidates rc WHERE rc.user_id=lp.user_id AND rc.status='pending'
		)
		ORDER BY lp.user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []radarReleaseUser
	for rows.Next() {
		var user radarReleaseUser
		var weekends int
		if err := rows.Scan(&user.ID, &user.ItemsPerDay, &weekends, &user.TimeZone); err != nil {
			return nil, err
		}
		user.WeekendsEnabled = weekends == 1
		users = append(users, user)
	}
	return users, rows.Err()
}

func (store *Store) releaseRadarForUser(
	ctx context.Context,
	user radarReleaseUser,
	now time.Time,
	passLimit int,
) (int, error) {
	location, err := time.LoadLocation(user.TimeZone)
	if err != nil {
		return 0, fmt.Errorf("load Radar time zone: %w", err)
	}
	localNow := now.In(location)
	if !user.WeekendsEnabled && (localNow.Weekday() == time.Saturday || localNow.Weekday() == time.Sunday) {
		return 0, nil
	}
	localDate := localNow.Format("2006-01-02")
	localDayStart := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location,
	).UTC()
	localDayEnd := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day()+1, 0, 0, 0, 0, location,
	).UTC()
	limit := passLimit
	if user.ItemsPerDay == 0 && limit > maximumUnlimitedReleasePass {
		limit = maximumUnlimitedReleasePass
	}
	if user.ItemsPerDay > 0 && limit > user.ItemsPerDay {
		limit = user.ItemsPerDay
	}
	if limit <= 0 {
		return 0, nil
	}

	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO radar_daily_usage (user_id,local_date)
		VALUES (?,?) ON CONFLICT(user_id,local_date) DO NOTHING`, user.ID, localDate); err != nil {
		return 0, err
	}
	if user.ItemsPerDay > 0 {
		var releasedCount int
		var lastReleased sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT released_count,last_released_at
			FROM radar_daily_usage WHERE user_id=? AND local_date=?`, user.ID, localDate).
			Scan(&releasedCount, &lastReleased); err != nil {
			return 0, err
		}
		if releasedCount >= user.ItemsPerDay {
			return 0, tx.Commit()
		}
		if lastReleased.Valid {
			last, parseErr := parseTimestamp(lastReleased.String)
			if parseErr != nil {
				return 0, parseErr
			}
			if last.Add(radarReleaseInterval(user.ItemsPerDay)).After(now) {
				return 0, tx.Commit()
			}
		}
		limit = 1
	}
	candidatePoolLimit := max(limit*8, 40)
	rows, err := tx.QueryContext(ctx, `SELECT rc.id,si.source_id,rc.relevance_score
		FROM radar_candidates rc
		JOIN source_items si ON si.id=rc.source_item_id
		WHERE rc.user_id=? AND rc.status='pending'
			AND rc.ranker_version=? AND rc.relevance_score>=?
			AND COALESCE(si.published_at,si.retrieved_at)>=?
		ORDER BY rc.relevance_score DESC,rc.created_at,rc.id LIMIT ?`,
		user.ID, radar.RankerVersion, radar.MinimumDeliveryScore,
		timestamp(now.Add(-radarCandidateMaxAge)), candidatePoolLimit)
	if err != nil {
		return 0, err
	}
	var candidatePool []radarReleaseCandidate
	for rows.Next() {
		var candidate radarReleaseCandidate
		if err := rows.Scan(&candidate.ID, &candidate.SourceID, &candidate.Score); err != nil {
			rows.Close()
			return 0, err
		}
		candidatePool = append(candidatePool, candidate)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	sourceUsage, err := radarSourceUsage(
		ctx, tx, user.ID, localDayStart, localDayEnd,
	)
	if err != nil {
		return 0, err
	}

	candidateIDs := diverseRadarCandidates(candidatePool, sourceUsage, limit)
	released := 0
	for _, candidateID := range candidateIDs {
		if user.ItemsPerDay > 0 {
			result, updateErr := tx.ExecContext(ctx, `UPDATE radar_daily_usage
				SET released_count=released_count+1,last_released_at=?,updated_at=?
				WHERE user_id=? AND local_date=? AND released_count<?`,
				timestamp(now), timestamp(now), user.ID, localDate, user.ItemsPerDay)
			if updateErr != nil {
				return 0, updateErr
			}
			changed, updateErr := result.RowsAffected()
			if updateErr != nil {
				return 0, updateErr
			}
			if changed == 0 {
				break
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE radar_daily_usage
				SET released_count=released_count+1,last_released_at=?,updated_at=?
				WHERE user_id=? AND local_date=?`,
				timestamp(now), timestamp(now), user.ID, localDate); err != nil {
				return 0, err
			}
		}
		result, err := tx.ExecContext(ctx, `UPDATE radar_candidates
			SET status='qualified',released_at=?,updated_at=? WHERE id=? AND user_id=? AND status='pending'`,
			timestamp(now), timestamp(now), candidateID, user.ID)
		if err != nil {
			return 0, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if changed == 0 {
			return 0, fmt.Errorf("reserve Radar candidate %s: %w", candidateID, sql.ErrNoRows)
		}
		payload, err := json.Marshal(map[string]string{"candidate_id": candidateID})
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO job_queue
			(id,user_id,job_type,due_at,idempotency_key,payload_json)
			VALUES (?,?,?,?,?,?) ON CONFLICT(idempotency_key) DO NOTHING`,
			mustID("job"), user.ID, "deliver_radar", timestamp(now),
			"radar-deliver:"+candidateID, string(payload))
		if err != nil {
			return 0, err
		}
		released++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return released, nil
}

func radarReleaseInterval(itemsPerDay int) time.Duration {
	if itemsPerDay <= 0 {
		return 0
	}
	return 24 * time.Hour / time.Duration(itemsPerDay)
}

func radarSourceUsage(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	dayStart time.Time,
	dayEnd time.Time,
) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT si.source_id,COUNT(*)
		FROM radar_candidates rc JOIN source_items si ON si.id=rc.source_item_id
		WHERE rc.user_id=? AND rc.released_at>=? AND rc.released_at<?
		GROUP BY si.source_id`, userID, timestamp(dayStart), timestamp(dayEnd))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	usage := map[string]int{}
	for rows.Next() {
		var sourceID string
		var count int
		if err := rows.Scan(&sourceID, &count); err != nil {
			return nil, err
		}
		usage[sourceID] = count
	}
	return usage, rows.Err()
}

func diverseRadarCandidates(pool []radarReleaseCandidate, historicalUsage map[string]int, limit int) []string {
	if limit <= 0 {
		return nil
	}
	ranked := append([]radarReleaseCandidate(nil), pool...)
	sort.SliceStable(ranked, func(left, right int) bool {
		leftScore := ranked[left].Score - math.Min(0.24, float64(historicalUsage[ranked[left].SourceID])*0.08)
		rightScore := ranked[right].Score - math.Min(0.24, float64(historicalUsage[ranked[right].SourceID])*0.08)
		return leftScore > rightScore
	})
	selected := make([]string, 0, min(limit, len(pool)))
	selectedIDs := make(map[string]bool, len(pool))
	perSource := make(map[string]int)
	for _, candidate := range ranked {
		if perSource[candidate.SourceID] >= 2 {
			continue
		}
		selected = append(selected, candidate.ID)
		selectedIDs[candidate.ID] = true
		perSource[candidate.SourceID]++
		if len(selected) == limit {
			return selected
		}
	}
	for _, candidate := range ranked {
		if selectedIDs[candidate.ID] {
			continue
		}
		selected = append(selected, candidate.ID)
		if len(selected) == limit {
			break
		}
	}
	return selected
}
