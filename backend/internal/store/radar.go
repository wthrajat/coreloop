package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/radar"
)

type SourceRecord struct {
	ID, Publisher, URL, FetchMethod, Role, AdapterConfig, ETag, LastModified string
	Tier                                                                     int
}

type SourcePollOutcome struct {
	NotModified    bool
	AttemptedItems int
	FailedItems    int
}

type RadarCandidate struct {
	ID, UserID, SourceItemID, TopicID, Publisher, SourceRole string
	Title, URL, Summary, Category, RankerVersion             string
	Discovery                                                []radar.SourceReference
	Score                                                    float64
	PublishedAt                                              time.Time
}

func (store *Store) Source(ctx context.Context, id string) (SourceRecord, error) {
	var source SourceRecord
	var etag, last sql.NullString
	err := store.database.QueryRowContext(ctx, `SELECT id,publisher,canonical_url,source_tier,fetch_method,
		source_role,adapter_config_json,etag,last_modified FROM sources WHERE id=? AND enabled=1`, id).
		Scan(&source.ID, &source.Publisher, &source.URL, &source.Tier, &source.FetchMethod,
			&source.Role, &source.AdapterConfig, &etag, &last)
	source.ETag, source.LastModified = etag.String, last.String
	return source, err
}

func (store *Store) SaveSourceItems(
	ctx context.Context,
	source SourceRecord,
	items []radar.Item,
	etag string,
	lastModified string,
	outcome SourcePollOutcome,
	now time.Time,
) ([]string, error) {
	valueGroups := make([]string, 0, len(items))
	arguments := make([]any, 0, len(items)*17)
	for _, item := range items {
		values, valid, err := normalizedSourceItemValues(source, item, etag, lastModified, now)
		if err != nil {
			return nil, err
		}
		if !valid {
			continue
		}
		valueGroups = append(valueGroups, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
		arguments = append(arguments, values...)
	}

	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var changedItems []string
	if len(valueGroups) > 0 {
		query := `INSERT INTO source_items
			(id,source_id,canonical_url,normalized_url,title,published_at,retrieved_at,
			 content_hash,evidence_json,cluster_key,etag,last_modified,category,
			 community_score,community_comments,community_signals_available,discovery_json)
			VALUES ` + strings.Join(valueGroups, ",") + `
			ON CONFLICT(canonical_url) DO UPDATE SET
				source_id=CASE WHEN
					excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.source_id ELSE source_items.source_id END,
				normalized_url=excluded.normalized_url,
				title=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.title ELSE source_items.title END,
				published_at=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN COALESCE(excluded.published_at,source_items.published_at)
					ELSE source_items.published_at END,
				retrieved_at=excluded.retrieved_at,
				content_hash=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.content_hash ELSE source_items.content_hash END,
				evidence_json=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.evidence_json ELSE source_items.evidence_json END,
				cluster_key=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.cluster_key ELSE source_items.cluster_key END,
				etag=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.etag ELSE source_items.etag END,
				last_modified=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.last_modified ELSE source_items.last_modified END,
				category=CASE WHEN excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
					THEN excluded.category ELSE source_items.category END,
				community_score=MAX(source_items.community_score,excluded.community_score),
				community_comments=MAX(source_items.community_comments,excluded.community_comments),
				community_signals_available=MAX(
					source_items.community_signals_available,
					excluded.community_signals_available),
				discovery_json=CASE WHEN excluded.discovery_json='[]'
					THEN source_items.discovery_json ELSE excluded.discovery_json END
			WHERE (source_items.content_hash<>excluded.content_hash AND (
					excluded.source_id=source_items.source_id OR
					(SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)))
				OR excluded.community_score>source_items.community_score
				OR excluded.community_comments>source_items.community_comments
				OR excluded.community_signals_available>
					source_items.community_signals_available
				OR (excluded.discovery_json<>'[]' AND
					excluded.discovery_json<>source_items.discovery_json)
				OR (SELECT source_tier FROM sources WHERE id=excluded.source_id)<
					(SELECT source_tier FROM sources WHERE id=source_items.source_id)
			RETURNING id`
		rows, err := tx.QueryContext(ctx, query, arguments...)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		for rows.Next() {
			var actualID string
			if err := rows.Scan(&actualID); err != nil {
				rows.Close()
				return nil, err
			}
			if !seen[actualID] {
				seen[actualID] = true
				changedItems = append(changedItems, actualID)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	acceptedItems := len(valueGroups)
	rejectedItems := len(items) - acceptedItems
	totalFailures := outcome.FailedItems + rejectedItems
	pollState := "healthy"
	var errorCode, errorSummary any
	if totalFailures > 0 {
		pollState = "degraded"
		errorCode = "partial_item_failure"
		errorSummary = fmt.Sprintf(
			"%d source items could not be fetched or accepted; %d usable items remained.",
			totalFailures,
			acceptedItems,
		)
	}
	_, err = tx.ExecContext(ctx, `UPDATE sources SET etag=?,last_modified=?,last_polled_at=?,
		consecutive_failures=0,last_poll_state=?,last_success_at=?,
		last_error_code=?,last_error_summary=?,last_error_at=CASE
			WHEN ? IS NULL THEN NULL ELSE ? END,
		last_item_count=CASE WHEN ?=1 THEN last_item_count ELSE ? END,updated_at=?
		WHERE id=?`, etag, lastModified, timestamp(now), pollState, timestamp(now),
		errorCode, errorSummary, errorCode, timestamp(now), boolInt(outcome.NotModified),
		acceptedItems, timestamp(now), source.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return changedItems, nil
}

func normalizedSourceItemValues(
	source SourceRecord,
	item radar.Item,
	etag string,
	lastModified string,
	now time.Time,
) ([]any, bool, error) {
	normalizedURL, err := radar.CanonicalURL(item.URL)
	if item.Title == "" || err != nil {
		return nil, false, nil
	}
	neutralTitle := radar.NeutralText(item.Title)
	if neutralTitle == "" {
		return nil, false, nil
	}
	neutralSummary := radar.NeutralText(item.Summary)
	category := radar.Classify(neutralTitle, neutralSummary)
	contentDigest := sha256.Sum256([]byte(
		normalizedURL + "\n" + neutralTitle + "\n" + neutralSummary,
	))
	published := any(nil)
	if !item.PublishedAt.IsZero() {
		published = timestamp(item.PublishedAt)
	}
	discovery, _ := json.Marshal(item.DiscoveredVia)
	evidence, _ := json.Marshal(map[string]any{
		"summary":                     neutralSummary,
		"community_points":            item.CommunityPoints,
		"community_comments":          item.CommunityComments,
		"community_signals_available": item.CommunitySignalsAvailable,
		"discovered_via":              item.DiscoveredVia,
	})
	itemID, err := ids.New("srcitem")
	if err != nil {
		return nil, false, err
	}
	return []any{
		itemID, source.ID, normalizedURL, normalizedURL, neutralTitle,
		published, timestamp(now), hex.EncodeToString(contentDigest[:]), string(evidence),
		radar.ClusterKey(normalizedURL, neutralTitle), etag, lastModified, string(category),
		item.CommunityPoints, item.CommunityComments, boolInt(item.CommunitySignalsAvailable),
		string(discovery),
	}, true, nil
}

func (store *Store) SourcePollFailed(
	ctx context.Context,
	id string,
	errorCode string,
	errorSummary string,
	now time.Time,
) error {
	if strings.TrimSpace(errorCode) == "" {
		errorCode = "source_poll_failed"
	}
	errorCode = boundedFailureText(errorCode, 64, "source_poll_failed")
	errorSummary = boundedFailureText(
		errorSummary,
		500,
		"The source could not be fetched or processed.",
	)
	_, err := store.database.ExecContext(ctx, `UPDATE sources SET
		consecutive_failures=consecutive_failures+1,last_polled_at=?,last_poll_state='failed',
		last_error_code=?,last_error_summary=?,last_error_at=?,updated_at=? WHERE id=?`,
		timestamp(now), errorCode, errorSummary, timestamp(now), timestamp(now), id)
	return err
}

type radarUserTopic struct {
	TopicID        string
	Terms          []string
	FeedbackWeight float64
}

type radarUser struct {
	ID     string
	Topics []radarUserTopic
}

type radarRankingItem struct {
	Candidate          RadarCandidate
	SourceTier         int
	CommunityPoints    int
	CommunityComments  int
	CommunityAvailable bool
}

type radarCandidateInsert struct {
	ID, UserID, SourceItemID, TopicID, Breakdown, State string
	Score                                               float64
	RejectionReason                                     any
}

func (store *Store) RankSourceItems(
	ctx context.Context,
	itemIDs []string,
	now time.Time,
) (int, error) {
	items, err := store.radarItemsForRanking(ctx, itemIDs, now)
	if err != nil {
		return 0, err
	}
	users, err := store.radarUsers(ctx)
	if err != nil {
		return 0, err
	}
	inserts := make([]radarCandidateInsert, 0, len(items)*len(users))
	for _, item := range items {
		category := radar.Category(item.Candidate.Category)
		for _, user := range users {
			var selectedTopic radarUserTopic
			var selectedBreakdown radar.ScoreBreakdown
			for _, topic := range user.Topics {
				breakdown := radar.CalculateScore(radar.ScoreInput{
					Title: item.Candidate.Title, Summary: item.Candidate.Summary,
					TopicTerms: topic.Terms, SourceTier: item.SourceTier,
					PublishedAgeHours: now.Sub(item.Candidate.PublishedAt).Hours(),
					Category:          category, CommunityPoints: item.CommunityPoints,
					CommunityComments:         item.CommunityComments,
					CommunitySignalsAvailable: item.CommunityAvailable,
					SourceRole:                item.Candidate.SourceRole,
				})
				if breakdown.Total > selectedBreakdown.Total {
					selectedTopic, selectedBreakdown = topic, breakdown
				}
			}
			if selectedTopic.TopicID == "" {
				continue
			}
			adjustedScore := math.Max(0, math.Min(1, selectedBreakdown.Total+selectedTopic.FeedbackWeight))
			selectedBreakdown.Total = math.Round(adjustedScore*1000) / 1000
			decision := radar.DecideEditorialEligibility(radar.ScoreInput{
				Title: item.Candidate.Title, Summary: item.Candidate.Summary,
				TopicTerms: selectedTopic.Terms, SourceTier: item.SourceTier,
				PublishedAgeHours: now.Sub(item.Candidate.PublishedAt).Hours(),
				Category:          category, CommunityPoints: item.CommunityPoints,
				CommunityComments:         item.CommunityComments,
				CommunitySignalsAvailable: item.CommunityAvailable,
				SourceRole:                item.Candidate.SourceRole,
			}, selectedBreakdown)
			breakdown, _ := json.Marshal(map[string]any{
				"ranker": radar.RankerVersion, "category": category,
				"scores": selectedBreakdown, "editorial_decision": decision,
			})
			state := "rejected"
			var rejectionReason any = decision.Reason
			if decision.Eligible {
				state = "pending"
				rejectionReason = nil
			}
			candidateID, err := ids.New("rad")
			if err != nil {
				return 0, err
			}
			inserts = append(inserts, radarCandidateInsert{
				ID: candidateID, UserID: user.ID,
				SourceItemID: item.Candidate.SourceItemID, TopicID: selectedTopic.TopicID,
				Breakdown: string(breakdown), State: state,
				Score: selectedBreakdown.Total, RejectionReason: rejectionReason,
			})
		}
	}
	return store.upsertRadarCandidates(ctx, inserts)
}

func (store *Store) radarItemsForRanking(
	ctx context.Context,
	itemIDs []string,
	now time.Time,
) ([]radarRankingItem, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(itemIDs)), ",")
	arguments := make([]any, len(itemIDs))
	for index, itemID := range itemIDs {
		arguments[index] = itemID
	}
	rows, err := store.database.QueryContext(ctx, `SELECT si.id,s.publisher,s.source_role,si.title,
		si.normalized_url,si.evidence_json,si.category,si.community_score,si.community_comments,
		si.community_signals_available,
		si.discovery_json,s.source_tier,si.published_at
		FROM source_items si JOIN sources s ON s.id=si.source_id
		WHERE si.id IN (`+placeholders+`)`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[string]radarRankingItem, len(itemIDs))
	for rows.Next() {
		var item radarRankingItem
		var published sql.NullString
		var evidence, discovery string
		var communityAvailable int
		if err := rows.Scan(
			&item.Candidate.SourceItemID, &item.Candidate.Publisher,
			&item.Candidate.SourceRole, &item.Candidate.Title, &item.Candidate.URL,
			&evidence, &item.Candidate.Category, &item.CommunityPoints,
			&item.CommunityComments, &communityAvailable, &discovery,
			&item.SourceTier, &published,
		); err != nil {
			return nil, err
		}
		item.CommunityAvailable = communityAvailable == 1
		item.Candidate.PublishedAt = now
		if published.Valid {
			if parsed, parseErr := parseTimestamp(published.String); parseErr == nil {
				item.Candidate.PublishedAt = parsed
			}
		}
		var evidenceValue struct {
			Summary string `json:"summary"`
		}
		_ = json.Unmarshal([]byte(evidence), &evidenceValue)
		item.Candidate.Summary = evidenceValue.Summary
		_ = json.Unmarshal([]byte(discovery), &item.Candidate.Discovery)
		byID[item.Candidate.SourceItemID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]radarRankingItem, 0, len(byID))
	for _, itemID := range itemIDs {
		if item, exists := byID[itemID]; exists {
			items = append(items, item)
		}
	}
	return items, nil
}

func (store *Store) upsertRadarCandidates(
	ctx context.Context,
	candidates []radarCandidateInsert,
) (int, error) {
	if len(candidates) == 0 {
		return 0, nil
	}
	values := strings.TrimSuffix(strings.Repeat(`(?,?,?,?,?,?,?,CASE
		WHEN EXISTS (SELECT 1 FROM radar_candidates previous
			WHERE previous.user_id=? AND previous.source_item_id=? AND previous.status='delivered')
			THEN 'delivered'
		WHEN EXISTS (SELECT 1 FROM radar_candidates previous
			WHERE previous.user_id=? AND previous.source_item_id=? AND previous.status='skipped')
			THEN 'skipped'
		ELSE ? END,?),`, len(candidates)), ",")
	arguments := make([]any, 0, len(candidates)*13)
	for _, candidate := range candidates {
		arguments = append(arguments,
			candidate.ID, candidate.UserID, candidate.SourceItemID, candidate.TopicID,
			radar.RankerVersion, candidate.Score, candidate.Breakdown,
			candidate.UserID, candidate.SourceItemID, candidate.UserID,
			candidate.SourceItemID, candidate.State, candidate.RejectionReason,
		)
	}
	rows, err := store.database.QueryContext(ctx, `INSERT INTO radar_candidates
		(id,user_id,source_item_id,topic_id,ranker_version,relevance_score,
		 score_breakdown_json,status,rejection_reason) VALUES `+values+`
		ON CONFLICT(user_id,source_item_id,ranker_version) DO UPDATE SET
			topic_id=excluded.topic_id,relevance_score=excluded.relevance_score,
			score_breakdown_json=excluded.score_breakdown_json,
			status=CASE WHEN radar_candidates.status IN ('delivered','skipped','qualified')
				THEN radar_candidates.status ELSE excluded.status END,
			rejection_reason=CASE
				WHEN radar_candidates.status IN ('delivered','skipped','qualified')
				THEN radar_candidates.rejection_reason ELSE excluded.rejection_reason END,
			updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		RETURNING status`, arguments...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	pending := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return 0, err
		}
		if status == "pending" {
			pending++
		}
	}
	return pending, rows.Err()
}

func (store *Store) radarUsers(ctx context.Context) ([]radarUser, error) {
	rows, err := store.database.QueryContext(ctx, `SELECT lp.user_id,t.id,t.title,
		t.objectives_json,utp.feedback_weight
		FROM learning_preferences lp
		JOIN user_topic_preferences utp ON utp.user_id=lp.user_id
		JOIN topics t ON t.id=utp.topic_id
		JOIN users u ON u.id=lp.user_id
		JOIN delivery_destinations dd ON dd.user_id=lp.user_id AND dd.channel='telegram'
			AND dd.enabled=1 AND dd.status='connected'
		WHERE lp.radar_enabled=1 AND utp.excluded=0 AND u.status='active'
		ORDER BY lp.user_id,utp.priority DESC,t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]*radarUser{}
	var order []string
	for rows.Next() {
		var userID, topicID, topicTitle, objectives string
		var feedbackWeight float64
		if err := rows.Scan(&userID, &topicID, &topicTitle, &objectives, &feedbackWeight); err != nil {
			return nil, err
		}
		var objectiveValues []string
		_ = json.Unmarshal([]byte(objectives), &objectiveValues)
		user := byID[userID]
		if user == nil {
			user = &radarUser{ID: userID}
			byID[userID] = user
			order = append(order, userID)
		}
		terms := append([]string{topicTitle}, objectiveValues...)
		user.Topics = append(user.Topics, radarUserTopic{
			TopicID: topicID, Terms: terms, FeedbackWeight: feedbackWeight,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	users := make([]radarUser, 0, len(order))
	for _, id := range order {
		users = append(users, *byID[id])
	}
	return users, nil
}

func (store *Store) CompleteRadar(ctx context.Context, userID, candidateID, state string, now time.Time) error {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE radar_candidates SET status=?,updated_at=?
		WHERE id=? AND user_id=? AND status<>?`, state, timestamp(now), candidateID,
		userID, state)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		var currentState string
		err = tx.QueryRowContext(ctx, `SELECT status FROM radar_candidates
			WHERE id=? AND user_id=?`, candidateID, userID).Scan(&currentState)
		if err != nil {
			return err
		}
		if currentState != state {
			return sql.ErrNoRows
		}
		return tx.Commit()
	}
	if state == "skipped" {
		_, err = tx.ExecContext(ctx, `UPDATE user_topic_preferences SET
			feedback_weight=MAX(-0.25,feedback_weight-0.03),updated_at=?
			WHERE user_id=? AND topic_id=(SELECT topic_id FROM radar_candidates WHERE id=?)`,
			timestamp(now), userID, candidateID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (store *Store) RejectRadar(
	ctx context.Context,
	userID string,
	candidateID string,
	reason string,
	now time.Time,
) error {
	result, err := store.database.ExecContext(ctx, `UPDATE radar_candidates
		SET status='rejected',rejection_reason=?,updated_at=?
		WHERE id=? AND user_id=? AND status IN ('pending','qualified')`,
		reason, timestamp(now), candidateID, userID)
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
	return nil
}

func (store *Store) RadarCandidate(ctx context.Context, candidateID string) (RadarCandidate, error) {
	var candidate RadarCandidate
	var published string
	var evidence, discovery string
	err := store.database.QueryRowContext(ctx, `SELECT rc.id,rc.user_id,rc.source_item_id,
		COALESCE(rc.topic_id,''),s.publisher,s.source_role,si.title,si.normalized_url,
		si.evidence_json,si.category,si.discovery_json,rc.relevance_score,
		COALESCE(si.published_at,si.retrieved_at),rc.ranker_version
		FROM radar_candidates rc JOIN source_items si ON si.id=rc.source_item_id
		JOIN sources s ON s.id=si.source_id
		WHERE rc.id=? AND rc.status='qualified'`, candidateID).
		Scan(&candidate.ID, &candidate.UserID, &candidate.SourceItemID, &candidate.TopicID,
			&candidate.Publisher, &candidate.SourceRole, &candidate.Title, &candidate.URL,
			&evidence, &candidate.Category, &discovery, &candidate.Score, &published,
			&candidate.RankerVersion)
	if err != nil {
		return candidate, err
	}
	var evidenceValue struct {
		Summary string `json:"summary"`
	}
	_ = json.Unmarshal([]byte(evidence), &evidenceValue)
	candidate.Summary = evidenceValue.Summary
	_ = json.Unmarshal([]byte(discovery), &candidate.Discovery)
	candidate.PublishedAt, _ = parseTimestamp(published)
	return candidate, nil
}
