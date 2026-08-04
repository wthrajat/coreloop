package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"time"

	"coreloop/backend/internal/radar"
)

type SourceRecord struct {
	ID, Publisher, URL, FetchMethod, ETag, LastModified string
	Tier                                                int
}
type RadarCandidate struct {
	ID, UserID, SourceItemID, TopicID, Publisher, Title, URL, Summary string
	Score                                                             float64
	PublishedAt                                                       time.Time
}

func (store *Store) Source(ctx context.Context, id string) (SourceRecord, error) {
	var source SourceRecord
	var etag, last sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT id,publisher,canonical_url,source_tier,fetch_method,etag,last_modified FROM sources WHERE id=? AND enabled=1`, id).Scan(&source.ID, &source.Publisher, &source.URL, &source.Tier, &source.FetchMethod, &etag, &last)
	source.ETag, source.LastModified = etag.String, last.String
	return source, err
}

func (store *Store) SaveSourceItems(ctx context.Context, source SourceRecord, items []radar.Item, etag, lastModified string, now time.Time) ([]string, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var inserted []string
	for _, item := range items {
		parsedURL, parseError := url.Parse(item.URL)
		if item.URL == "" || item.Title == "" || parseError != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			continue
		}
		sum := sha256.Sum256([]byte(item.Title + "\n" + item.Summary))
		hash := hex.EncodeToString(sum[:])
		id := mustID("srcitem")
		published := any(nil)
		if !item.PublishedAt.IsZero() {
			published = timestamp(item.PublishedAt)
		}
		evidence, _ := json.Marshal(map[string]string{"summary": item.Summary})
		result, err := tx.ExecContext(ctx, `INSERT INTO source_items
			(id,source_id,canonical_url,title,published_at,retrieved_at,content_hash,evidence_json,etag,last_modified)
			VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(canonical_url) DO UPDATE SET title=excluded.title,published_at=excluded.published_at,retrieved_at=excluded.retrieved_at,content_hash=excluded.content_hash,evidence_json=excluded.evidence_json,etag=excluded.etag,last_modified=excluded.last_modified
			WHERE source_items.content_hash<>excluded.content_hash`, id, source.ID, item.URL, item.Title, published, timestamp(now), hash, string(evidence), etag, lastModified)
		if err != nil {
			return nil, err
		}
		changed, _ := result.RowsAffected()
		if changed > 0 {
			var actual string
			if err := tx.QueryRowContext(ctx, "SELECT id FROM source_items WHERE canonical_url=?", item.URL).Scan(&actual); err != nil {
				return nil, err
			}
			inserted = append(inserted, actual)
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE sources SET etag=?,last_modified=?,last_polled_at=?,consecutive_failures=0,updated_at=? WHERE id=?`, etag, lastModified, timestamp(now), timestamp(now), source.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inserted, nil
}

func (store *Store) SourcePollFailed(ctx context.Context, id string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE sources SET consecutive_failures=consecutive_failures+1,last_polled_at=?,updated_at=? WHERE id=?`, timestamp(now), timestamp(now), id)
	return err
}

func (store *Store) RankSourceItem(ctx context.Context, itemID string, now time.Time) ([]RadarCandidate, error) {
	var title, url, publisher, evidence string
	var tier int
	var publishedNull sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT si.title,si.canonical_url,s.publisher,s.source_tier,si.evidence_json,si.published_at FROM source_items si JOIN sources s ON s.id=si.source_id WHERE si.id=?`, itemID).Scan(&title, &url, &publisher, &tier, &evidence, &publishedNull)
	if err != nil {
		return nil, err
	}
	publishedTime := now
	if publishedNull.Valid {
		publishedTime, _ = parseTimestamp(publishedNull.String)
	}
	var evidenceValue map[string]string
	_ = json.Unmarshal([]byte(evidence), &evidenceValue)
	summary := evidenceValue["summary"]
	rows, err := store.database.QueryContext(ctx, `SELECT lp.user_id,t.id,t.title,t.objectives_json,utp.feedback_weight
		FROM learning_preferences lp
		JOIN user_topic_preferences utp ON utp.user_id=lp.user_id
		JOIN topics t ON t.id=utp.topic_id
		JOIN users u ON u.id=lp.user_id
		JOIN delivery_destinations dd ON dd.user_id=lp.user_id AND dd.channel='telegram'
			AND dd.enabled=1 AND dd.status='connected'
		WHERE lp.radar_enabled=1 AND utp.excluded=0 AND u.status='active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []RadarCandidate
	for rows.Next() {
		var userID, topicID, topicTitle, objectives string
		var feedbackWeight float64
		if err := rows.Scan(&userID, &topicID, &topicTitle, &objectives, &feedbackWeight); err != nil {
			return nil, err
		}
		var objectiveValues []string
		_ = json.Unmarshal([]byte(objectives), &objectiveValues)
		terms := append([]string{topicTitle}, objectiveValues...)
		score := radar.Score(title, summary, terms, tier, now.Sub(publishedTime).Hours()) + feedbackWeight
		if score > 1 {
			score = 1
		}
		if score < 0 {
			score = 0
		}
		if score < 0.58 {
			continue
		}
		candidateID := mustID("rad")
		breakdown, _ := json.Marshal(map[string]any{"score": score, "ranker": radar.RankerVersion})
		_, err := store.database.ExecContext(ctx, `INSERT INTO radar_candidates
			(id,user_id,source_item_id,topic_id,ranker_version,relevance_score,score_breakdown_json,status)
			VALUES (?,?,?,?,?,?,?,'qualified') ON CONFLICT(user_id,source_item_id,ranker_version) DO NOTHING`, candidateID, userID, itemID, topicID, radar.RankerVersion, score, string(breakdown))
		if err != nil {
			return nil, err
		}
		var actualID, status string
		err = store.database.QueryRowContext(ctx, `SELECT id,status FROM radar_candidates WHERE user_id=? AND source_item_id=? AND ranker_version=?`, userID, itemID, radar.RankerVersion).Scan(&actualID, &status)
		if err != nil {
			return nil, err
		}
		if status == "qualified" {
			candidates = append(candidates, RadarCandidate{ID: actualID, UserID: userID, SourceItemID: itemID, TopicID: topicID, Publisher: publisher, Title: title, URL: url, Summary: summary, Score: score, PublishedAt: publishedTime})
		}
	}
	return candidates, rows.Err()
}

func (store *Store) CompleteRadar(ctx context.Context, userID, candidateID, state string, now time.Time) error {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE radar_candidates SET status=?,updated_at=? WHERE id=? AND user_id=?`, state, timestamp(now), candidateID, userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return sql.ErrNoRows
	}
	if state == "skipped" {
		_, err = tx.ExecContext(ctx, `UPDATE user_topic_preferences SET feedback_weight=MAX(-0.25,feedback_weight-0.03),updated_at=? WHERE user_id=? AND topic_id=(SELECT topic_id FROM radar_candidates WHERE id=?)`, timestamp(now), userID, candidateID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (store *Store) RadarCandidate(ctx context.Context, candidateID string) (RadarCandidate, error) {
	var candidate RadarCandidate
	var published sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT rc.id,rc.user_id,rc.source_item_id,COALESCE(rc.topic_id,''),s.publisher,si.title,si.canonical_url,si.evidence_json,rc.relevance_score,si.published_at FROM radar_candidates rc JOIN source_items si ON si.id=rc.source_item_id JOIN sources s ON s.id=si.source_id WHERE rc.id=? AND rc.status='qualified'`, candidateID).Scan(&candidate.ID, &candidate.UserID, &candidate.SourceItemID, &candidate.TopicID, &candidate.Publisher, &candidate.Title, &candidate.URL, &candidate.Summary, &candidate.Score, &published)
	if err != nil {
		return candidate, err
	}
	var evidence map[string]string
	if json.Unmarshal([]byte(candidate.Summary), &evidence) == nil {
		candidate.Summary = evidence["summary"]
	}
	if published.Valid {
		candidate.PublishedAt, _ = parseTimestamp(published.String)
	}
	return candidate, nil
}
