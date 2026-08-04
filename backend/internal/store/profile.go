package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"coreloop/backend/internal/content"
	"coreloop/backend/internal/ids"
)

func (store *Store) Topics(ctx context.Context) ([]Topic, error) {
	rows, err := store.database.QueryContext(ctx, `SELECT id, slug, title, lane, difficulty, objectives_json
		FROM topics WHERE status='active' ORDER BY lane, title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []Topic
	for rows.Next() {
		var topic Topic
		var objectives string
		if err := rows.Scan(&topic.ID, &topic.Slug, &topic.Title, &topic.Lane, &topic.Difficulty, &objectives); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(objectives), &topic.Objectives); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

func (store *Store) Profile(ctx context.Context, userID string) (LearningProfile, Preferences, error) {
	var profile LearningProfile
	var goals, roles, current, target string
	err := store.database.QueryRowContext(ctx, `SELECT current_level, goals_json, target_roles_json,
		current_technologies_json, target_technologies_json FROM learning_profiles WHERE user_id=?`, userID).
		Scan(&profile.CurrentLevel, &goals, &roles, &current, &target)
	if err != nil {
		return LearningProfile{}, Preferences{}, err
	}
	for _, item := range []struct {
		source      string
		destination *[]string
	}{
		{goals, &profile.Goals}, {roles, &profile.TargetRoles},
		{current, &profile.CurrentTechnologies}, {target, &profile.TargetTechnologies},
	} {
		if err := json.Unmarshal([]byte(item.source), item.destination); err != nil {
			return LearningProfile{}, Preferences{}, err
		}
	}
	preferences, err := store.Preferences(ctx, userID)
	return profile, preferences, err
}

func (store *Store) Preferences(ctx context.Context, userID string) (Preferences, error) {
	var preferences Preferences
	var radar, radarWeekends, weekends int
	var paused sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT lesson_minutes, explanation_depth, lessons_per_day,
		radar_enabled, radar_items_per_day, radar_weekends_enabled, recall_mode, weekends_enabled,
		bundle_mode, time_zone, paused_until
		FROM learning_preferences WHERE user_id=?`, userID).Scan(&preferences.LessonMinutes,
		&preferences.ExplanationDepth, &preferences.LessonsPerDay, &radar, &preferences.RadarItemsPerDay,
		&radarWeekends, &preferences.RecallMode, &weekends, &preferences.BundleMode, &preferences.TimeZone, &paused)
	if err != nil {
		return Preferences{}, err
	}
	preferences.RadarEnabled = radar == 1
	preferences.RadarWeekendsEnabled = radarWeekends == 1
	preferences.WeekendsEnabled = weekends == 1
	if paused.Valid {
		value, err := parseTimestamp(paused.String)
		if err != nil {
			return Preferences{}, err
		}
		preferences.PausedUntil = &value
	}
	rows, err := store.database.QueryContext(ctx, `SELECT DISTINCT local_time FROM delivery_schedules
		WHERE user_id=? AND enabled=1 ORDER BY local_time`, userID)
	if err != nil {
		return Preferences{}, err
	}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return Preferences{}, err
		}
		preferences.DeliveryTimes = append(preferences.DeliveryTimes, value)
	}
	rows.Close()
	topicRows, err := store.database.QueryContext(ctx, `SELECT topic_id FROM user_topic_preferences
		WHERE user_id=? AND excluded=0 ORDER BY priority DESC, topic_id`, userID)
	if err != nil {
		return Preferences{}, err
	}
	defer topicRows.Close()
	for topicRows.Next() {
		var id string
		if err := topicRows.Scan(&id); err != nil {
			return Preferences{}, err
		}
		preferences.TopicIDs = append(preferences.TopicIDs, id)
	}
	return preferences, topicRows.Err()
}

func (store *Store) UpdateProfile(ctx context.Context, userID string, profile LearningProfile) error {
	_, err := store.database.ExecContext(ctx, `UPDATE learning_profiles SET current_level=?, goals_json=?,
		target_roles_json=?, current_technologies_json=?, target_technologies_json=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE user_id=?`, profile.CurrentLevel,
		encodeStrings(profile.Goals), encodeStrings(profile.TargetRoles), encodeStrings(profile.CurrentTechnologies),
		encodeStrings(profile.TargetTechnologies), userID)
	return err
}

func (store *Store) UpdatePreferences(ctx context.Context, userID string, preferences Preferences) error {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var paused any
	if preferences.PausedUntil != nil {
		paused = timestamp(*preferences.PausedUntil)
	}
	_, err = tx.ExecContext(ctx, `UPDATE learning_preferences SET lesson_minutes=?, explanation_depth=?,
		lessons_per_day=?, radar_enabled=?, radar_items_per_day=?, radar_weekends_enabled=?, recall_mode=?,
		weekends_enabled=?, bundle_mode=?, time_zone=?, paused_until=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE user_id=?`,
		preferences.LessonMinutes, preferences.ExplanationDepth, preferences.LessonsPerDay,
		boolInt(preferences.RadarEnabled), preferences.RadarItemsPerDay,
		boolInt(preferences.RadarWeekendsEnabled), preferences.RecallMode,
		boolInt(preferences.WeekendsEnabled), preferences.BundleMode, preferences.TimeZone, paused, userID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM delivery_schedules WHERE user_id=?", userID); err != nil {
		return err
	}
	lastDay := 5
	if preferences.WeekendsEnabled {
		lastDay = 7
	}
	for day := 1; day <= lastDay; day++ {
		actualDay := day % 7
		for _, localTime := range preferences.DeliveryTimes {
			_, err := tx.ExecContext(ctx, `INSERT INTO delivery_schedules
				(id, user_id, day_of_week, local_time, time_zone) VALUES (?, ?, ?, ?, ?)`,
				mustID("sch"), userID, actualDay, localTime, preferences.TimeZone)
			if err != nil {
				return err
			}
		}
	}
	if len(preferences.TopicIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE user_topic_preferences SET excluded=1,
			updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE user_id=?`, userID); err != nil {
			return err
		}
		for _, topicID := range preferences.TopicIDs {
			result, err := tx.ExecContext(ctx, `UPDATE user_topic_preferences SET excluded=0,
				updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE user_id=? AND topic_id=?`, userID, topicID)
			if err != nil {
				return err
			}
			changed, _ := result.RowsAffected()
			if changed != 1 {
				return fmt.Errorf("unknown topic %q", topicID)
			}
		}
	}
	return tx.Commit()
}

func (store *Store) Overview(ctx context.Context, userID string, now time.Time) (Overview, error) {
	var overview Overview
	var theme, topic sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT tb.title, t.title FROM theme_blocks tb
		JOIN topics t ON t.id=tb.topic_id WHERE tb.user_id=? AND tb.status='active' LIMIT 1`, userID).Scan(&theme, &topic)
	if err != nil && err != sql.ErrNoRows {
		return Overview{}, err
	}
	overview.ActiveTheme, overview.ActiveTopic = theme.String, topic.String
	_ = store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM lesson_assignments
		WHERE user_id=? AND state IN ('queued','delivered')`, userID).Scan(&overview.UnreadLessons)
	_ = store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM lesson_assignments
		WHERE user_id=? AND state='read'`, userID).Scan(&overview.ReadLessons)
	var connected int
	_ = store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM delivery_destinations
		WHERE user_id=? AND enabled=1 AND status='connected'`, userID).Scan(&connected)
	overview.TelegramConnected = connected > 0
	_ = store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_queue WHERE user_id=? AND state='blocked_quota'`, userID).Scan(&overview.QuotaBlockedCount)
	if overview.QuotaBlockedCount > 0 {
		overview.QueueState = "quota_exhausted"
	} else {
		overview.QueueState = "healthy"
	}
	next, err := store.nextDelivery(ctx, userID, now)
	if err != nil {
		return Overview{}, err
	}
	overview.NextDeliveryAt = next
	return overview, nil
}

func (store *Store) nextDelivery(ctx context.Context, userID string, now time.Time) (*time.Time, error) {
	rows, err := store.database.QueryContext(ctx, `SELECT day_of_week, local_time, time_zone FROM delivery_schedules
		WHERE user_id=? AND enabled=1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []time.Time
	for rows.Next() {
		var day int
		var localTime, zone string
		if err := rows.Scan(&day, &localTime, &zone); err != nil {
			return nil, err
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			return nil, err
		}
		localNow := now.In(location)
		var hour, minute int
		if _, err := fmt.Sscanf(localTime, "%02d:%02d", &hour, &minute); err != nil {
			return nil, err
		}
		for offset := 0; offset <= 7; offset++ {
			date := localNow.AddDate(0, 0, offset)
			if int(date.Weekday()) != day {
				continue
			}
			candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, location)
			if candidate.After(localNow) {
				candidates = append(candidates, candidate.UTC())
				break
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].Before(candidates[right]) })
	return &candidates[0], nil
}

func (store *Store) Assignments(ctx context.Context, userID string, limit int) ([]AssignmentSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := store.database.QueryContext(ctx, `SELECT la.id, l.title, t.title, la.state,
		la.assigned_at, la.delivered_at, la.read_at FROM lesson_assignments la
		JOIN lessons l ON l.id=la.lesson_id JOIN topics t ON t.id=l.topic_id
		WHERE la.user_id=? ORDER BY la.assigned_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var assignments []AssignmentSummary
	for rows.Next() {
		var item AssignmentSummary
		var assigned string
		var delivered, read sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.Topic, &item.State, &assigned, &delivered, &read); err != nil {
			return nil, err
		}
		item.AssignedAt, err = parseTimestamp(assigned)
		if err != nil {
			return nil, err
		}
		if delivered.Valid {
			parsed, err := parseTimestamp(delivered.String)
			if err != nil {
				return nil, err
			}
			item.DeliveredAt = &parsed
		}
		if read.Valid {
			parsed, err := parseTimestamp(read.String)
			if err != nil {
				return nil, err
			}
			item.ReadAt = &parsed
		}
		assignments = append(assignments, item)
	}
	return assignments, rows.Err()
}

func (store *Store) MarkAssignment(ctx context.Context, userID, assignmentID, action string, now time.Time) error {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := "UPDATE lesson_assignments SET state='skipped' WHERE id=? AND user_id=?"
	arguments := []any{assignmentID, userID}
	if action == "read" {
		query = "UPDATE lesson_assignments SET state='read',read_at=COALESCE(read_at,?) WHERE id=? AND user_id=?"
		arguments = []any{timestamp(now), assignmentID, userID}
	}
	result, err := tx.ExecContext(ctx, query, arguments...)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return sql.ErrNoRows
	}
	interactionID, err := ids.New("act")
	if err != nil {
		return err
	}
	interactionResult, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO interactions
		(id, user_id, assignment_id, action, idempotency_key) VALUES (?, ?, ?, ?, ?)`,
		interactionID, userID, assignmentID, action, userID+":"+assignmentID+":"+action)
	if err != nil {
		return err
	}
	inserted, _ := interactionResult.RowsAffected()
	if action == "read" && inserted == 1 {
		var mode, encoded string
		err := tx.QueryRowContext(ctx, `SELECT lp.recall_mode,l.normalized_content_json
			FROM learning_preferences lp JOIN lesson_assignments la ON la.user_id=lp.user_id
			JOIN lessons l ON l.id=la.lesson_id WHERE la.id=? AND la.user_id=?`, assignmentID, userID).Scan(&mode, &encoded)
		if err != nil {
			return err
		}
		if mode != "off" {
			var draft content.LessonDraft
			if json.Unmarshal([]byte(encoded), &draft) == nil && strings.TrimSpace(draft.RecallQuestion) != "" {
				delay := 72 * time.Hour
				if mode == "standard" {
					delay = 24 * time.Hour
				}
				_, err = tx.ExecContext(ctx, `INSERT INTO reviews
					(id,user_id,assignment_id,question_text,expected_answer_json,due_at,state)
					VALUES (?,?,?,?,?,?,'scheduled')`, mustID("rev"), userID, assignmentID,
					draft.RecallQuestion, `{"evaluation":"self_assessed"}`, timestamp(now.Add(delay)))
				if err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

func (store *Store) DueRecallCount(ctx context.Context, userID string, now time.Time) (int, error) {
	var count int
	err := store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews WHERE user_id=? AND state IN ('scheduled','due') AND due_at<=?`, userID, timestamp(now)).Scan(&count)
	return count, err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
