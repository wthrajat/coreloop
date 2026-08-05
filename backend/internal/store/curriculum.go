package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"coreloop/backend/internal/content"
	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/securehash"
)

var ErrIncompleteLesson = errors.New("lesson content is incomplete")

type LessonPlan struct {
	UserID        string
	ThemeBlockID  string
	Topic         Topic
	Position      int
	Prerequisites []string
	Profile       LearningProfile
	Preferences   Preferences
}

func (store *Store) PlanNextLesson(ctx context.Context, userID string, now time.Time) (LessonPlan, error) {
	profile, preferences, err := store.Profile(ctx, userID)
	if err != nil {
		return LessonPlan{}, err
	}
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return LessonPlan{}, err
	}
	defer tx.Rollback()
	var themeID, topicID string
	var position int
	err = tx.QueryRowContext(ctx, `SELECT tb.id, tb.topic_id, COUNT(la.id)+1
		FROM theme_blocks tb LEFT JOIN lesson_assignments la ON la.theme_block_id=tb.id
		WHERE tb.user_id=? AND tb.status='active' GROUP BY tb.id, tb.topic_id`, userID).Scan(&themeID, &topicID, &position)
	if err == nil && position > 6 {
		if _, err := tx.ExecContext(ctx, `UPDATE theme_blocks SET status='completed', completed_at=?, updated_at=? WHERE id=?`, timestamp(now), timestamp(now), themeID); err != nil {
			return LessonPlan{}, err
		}
		err = sql.ErrNoRows
	}
	if err == sql.ErrNoRows {
		if err := tx.QueryRowContext(ctx, `SELECT utp.topic_id FROM user_topic_preferences utp
			JOIN topics t ON t.id=utp.topic_id
			LEFT JOIN theme_blocks tb ON tb.user_id=utp.user_id AND tb.topic_id=utp.topic_id
			WHERE utp.user_id=? AND utp.excluded=0 AND t.status='active'
			GROUP BY utp.topic_id ORDER BY COALESCE(MAX(tb.created_at), '0000') ASC, utp.priority DESC, t.title LIMIT 1`, userID).Scan(&topicID); err != nil {
			return LessonPlan{}, err
		}
		pathID, err := activePath(ctx, tx, userID)
		if err != nil {
			return LessonPlan{}, err
		}
		var sequence int
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence_number),0)+1 FROM theme_blocks WHERE learning_path_id=?", pathID).Scan(&sequence); err != nil {
			return LessonPlan{}, err
		}
		themeID, err = ids.New("thm")
		if err != nil {
			return LessonPlan{}, err
		}
		var title string
		if err := tx.QueryRowContext(ctx, "SELECT title FROM topics WHERE id=?", topicID).Scan(&title); err != nil {
			return LessonPlan{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO theme_blocks
			(id,user_id,learning_path_id,topic_id,sequence_number,title,objectives_json,planned_lesson_count,status,selection_reason,started_at)
			SELECT ?,?,?,?, ?, title, objectives_json, 6, 'active', 'least recently studied selected topic', ? FROM topics WHERE id=?`,
			themeID, userID, pathID, topicID, sequence, timestamp(now), topicID)
		if err != nil {
			return LessonPlan{}, err
		}
		position = 1
	} else if err != nil {
		return LessonPlan{}, err
	}
	var topic Topic
	var objectives, prerequisites string
	if err := tx.QueryRowContext(ctx, `SELECT id,slug,title,lane,difficulty,objectives_json,prerequisites_json FROM topics WHERE id=?`, topicID).
		Scan(&topic.ID, &topic.Slug, &topic.Title, &topic.Lane, &topic.Difficulty, &objectives, &prerequisites); err != nil {
		return LessonPlan{}, err
	}
	if err := json.Unmarshal([]byte(objectives), &topic.Objectives); err != nil {
		return LessonPlan{}, err
	}
	var prerequisiteList []string
	if err := json.Unmarshal([]byte(prerequisites), &prerequisiteList); err != nil {
		return LessonPlan{}, err
	}
	if err := tx.Commit(); err != nil {
		return LessonPlan{}, err
	}
	return LessonPlan{
		UserID: userID, ThemeBlockID: themeID, Topic: topic, Position: position,
		Prerequisites: prerequisiteList, Profile: profile, Preferences: preferences,
	}, nil
}

func activePath(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM learning_paths WHERE user_id=? AND status='active'", userID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id, err = ids.New("pth")
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO learning_paths (id,user_id,rationale) VALUES (?,?,?)`, id, userID, "Continuous usefulness-first curriculum across selected topics")
	return id, err
}

func (plan LessonPlan) Context() content.LessonContext {
	allObjectives := append([]string(nil), plan.Topic.Objectives...)
	objectives := allObjectives
	if len(objectives) > 0 {
		objectives = []string{objectives[(plan.Position-1)%len(objectives)]}
	}
	covered := make([]string, 0, len(allObjectives))
	seen := make(map[string]struct{}, len(allObjectives))
	for position := 1; position < plan.Position && len(allObjectives) > 0; position++ {
		objective := allObjectives[(position-1)%len(allObjectives)]
		if _, exists := seen[objective]; exists {
			continue
		}
		seen[objective] = struct{}{}
		covered = append(covered, objective)
	}
	return content.LessonContext{TopicID: plan.Topic.ID, Topic: plan.Topic.Title, Level: plan.Profile.CurrentLevel,
		Minutes: plan.Preferences.LessonMinutes, Depth: plan.Preferences.ExplanationDepth, ThemeID: plan.ThemeBlockID,
		Position: plan.Position, Objectives: objectives,
		Prerequisites: append([]string(nil), plan.Prerequisites...), CoveredObjectives: covered}
}

func (store *Store) SaveGeneratedLesson(ctx context.Context, plan LessonPlan, generated content.Generated, parts []string, cacheKey string, now time.Time) (string, string, error) {
	if !content.DeliveryReady(generated.Draft, plan.Preferences.LessonMinutes) ||
		!lessonPartsReady(parts) {
		return "", "", ErrIncompleteLesson
	}
	encoded, err := json.Marshal(generated.Draft)
	if err != nil {
		return "", "", err
	}
	fingerprint := securehash.SHA256(string(encoded))
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var lessonID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM lessons WHERE cache_key=? AND generation_state IN ('validated','published') ORDER BY version DESC LIMIT 1`, cacheKey).Scan(&lessonID)
	if err == sql.ErrNoRows {
		lessonID, err = ids.New("lsn")
		if err != nil {
			return "", "", err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO lessons
			(id,topic_id,prompt_version_id,lesson_type,title,estimated_reading_minutes,normalized_content_json,
			content_fingerprint,cache_key,verification_state,generation_state,generated_at,published_at)
			VALUES (?,?,?,'foundation',?,?,?,?,?,?,'validated',?,?)`,
			lessonID, plan.Topic.ID, content.PromptRecordID, generated.Draft.Title,
			generated.Draft.EstimatedMinutes, string(encoded), fingerprint, cacheKey,
			generated.VerificationState, timestamp(now), timestamp(now))
		if err != nil {
			return "", "", err
		}
		if err := insertLessonParts(ctx, tx, lessonID, parts); err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	}
	assignmentID, err := ids.New("asn")
	if err != nil {
		return "", "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO lesson_assignments
		(id,user_id,lesson_id,theme_block_id,schedule_position,assignment_reason,state,assigned_at)
		VALUES (?,?,?,?,?,'new','queued',?) ON CONFLICT(user_id,theme_block_id,schedule_position) DO NOTHING`,
		assignmentID, plan.UserID, lessonID, plan.ThemeBlockID, plan.Position, timestamp(now))
	if err != nil {
		return "", "", err
	}
	var storedAssignment string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM lesson_assignments WHERE user_id=? AND theme_block_id=? AND schedule_position=?`, plan.UserID, plan.ThemeBlockID, plan.Position).Scan(&storedAssignment); err != nil {
		return "", "", err
	}
	if err := attachDueRecall(ctx, tx, plan.UserID, storedAssignment, now); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return lessonID, storedAssignment, nil
}

func lessonPartsReady(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		count := len([]rune(strings.TrimSpace(part)))
		if count == 0 || count > 4096 {
			return false
		}
	}
	return true
}

func attachDueRecall(
	ctx context.Context,
	execer sqlExecer,
	userID string,
	assignmentID string,
	now time.Time,
) error {
	_, err := execer.ExecContext(ctx, `UPDATE lesson_assignments
		SET recall_review_id=(SELECT r.id FROM reviews r
			WHERE r.user_id=? AND r.state IN ('scheduled','due') AND r.due_at<=?
			AND NOT EXISTS (SELECT 1 FROM lesson_assignments used
				WHERE used.recall_review_id=r.id)
			ORDER BY r.due_at,r.id LIMIT 1)
		WHERE id=? AND user_id=? AND recall_review_id IS NULL`,
		userID, timestamp(now), assignmentID, userID)
	return err
}

func insertLessonParts(ctx context.Context, tx *sql.Tx, lessonID string, parts []string) error {
	if len(parts) == 0 {
		return nil
	}
	values := strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?,?),", len(parts)), ",")
	arguments := make([]any, 0, len(parts)*7)
	for index, part := range parts {
		partID, err := ids.New("part")
		if err != nil {
			return err
		}
		arguments = append(
			arguments, partID, lessonID, index+1, len(parts), part,
			len([]rune(part)), "telegram-html-v1",
		)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO lesson_parts
		(id,lesson_id,sequence_number,total_parts,rendered_text,character_count,renderer_version)
		VALUES `+values, arguments...)
	return err
}

func (store *Store) CachedLesson(ctx context.Context, cacheKey string) (content.LessonDraft, string, []string, error) {
	var draft content.LessonDraft
	var id, encoded string
	err := store.database.QueryRowContext(ctx, `SELECT id,normalized_content_json FROM lessons WHERE cache_key=? AND generation_state IN ('validated','published') ORDER BY version DESC LIMIT 1`, cacheKey).Scan(&id, &encoded)
	if err != nil {
		return draft, "", nil, err
	}
	if err := json.Unmarshal([]byte(encoded), &draft); err != nil {
		return draft, "", nil, err
	}
	rows, err := store.database.QueryContext(ctx, `SELECT rendered_text FROM lesson_parts WHERE lesson_id=? ORDER BY sequence_number`, id)
	if err != nil {
		return draft, "", nil, err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var part string
		if err := rows.Scan(&part); err != nil {
			return draft, "", nil, err
		}
		parts = append(parts, part)
	}
	return draft, id, parts, rows.Err()
}

func (store *Store) AssignCachedLesson(ctx context.Context, plan LessonPlan, lessonID string, now time.Time) (string, error) {
	id, err := ids.New("asn")
	if err != nil {
		return "", err
	}
	_, err = store.database.ExecContext(ctx, `INSERT INTO lesson_assignments
		(id,user_id,lesson_id,theme_block_id,schedule_position,assignment_reason,state,assigned_at)
		VALUES (?,?,?,?,?,'new','queued',?) ON CONFLICT(user_id,theme_block_id,schedule_position) DO NOTHING`,
		id, plan.UserID, lessonID, plan.ThemeBlockID, plan.Position, timestamp(now))
	if err != nil {
		return "", err
	}
	err = store.database.QueryRowContext(ctx, `SELECT id FROM lesson_assignments WHERE user_id=? AND theme_block_id=? AND schedule_position=?`, plan.UserID, plan.ThemeBlockID, plan.Position).Scan(&id)
	if err != nil {
		return "", err
	}
	if err := attachDueRecall(ctx, store.database, plan.UserID, id, now); err != nil {
		return "", err
	}
	return id, err
}

func (store *Store) AssignmentRecallQuestion(
	ctx context.Context,
	userID string,
	assignmentID string,
) (string, error) {
	var question string
	err := store.database.QueryRowContext(ctx, `SELECT r.question_text
		FROM lesson_assignments la JOIN reviews r ON r.id=la.recall_review_id
		WHERE la.id=? AND la.user_id=?`, assignmentID, userID).Scan(&question)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return question, err
}
