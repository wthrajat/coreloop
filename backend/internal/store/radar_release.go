package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"coreloop/backend/internal/ids"
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
	PendingItems    int
}

type radarReleaseCandidate struct {
	ID, SourceID, SourceFamily string
	URL, DiscoveryJSON         string
	Score                      float64
}

type RadarReleaseReport struct {
	ProfilesChecked   int
	Released          int
	WeekendPaused     int
	DailyTargetMet    int
	WaitingForSlot    int
	NoEligibleContent int
}

const (
	radarReleaseCompleted       = "released"
	radarReleaseWeekendPaused   = "weekend_paused"
	radarReleaseDailyTargetMet  = "daily_target_met"
	radarReleaseWaitingForSlot  = "waiting_for_slot"
	radarReleaseNoEligibleItems = "no_eligible_content"
)

// ReleaseRadarCandidates atomically reserves per-user daily slots and creates
// durable delivery jobs. Candidate selection and job creation share a
// transaction so a process failure cannot leave a qualified item without a
// wakeable job.
func (store *Store) ReleaseRadarCandidates(
	ctx context.Context,
	now time.Time,
) (RadarReleaseReport, error) {
	var report RadarReleaseReport
	if _, err := store.database.ExecContext(ctx, `UPDATE radar_candidates
		SET status='rejected',rejection_reason=CASE
			WHEN ranker_version<>? THEN 'superseded_ranking_policy'
			WHEN relevance_score<? THEN 'below_editorial_threshold'
			ELSE 'expired_before_release' END,
			updated_at=?
		WHERE status='pending' AND (
			ranker_version<>? OR relevance_score<? OR source_item_id IN (
				SELECT id FROM source_items WHERE COALESCE(published_at,retrieved_at)<?
			)
		)`, radar.RankerVersion, radar.MinimumDeliveryScore, timestamp(now),
		radar.RankerVersion, radar.MinimumDeliveryScore,
		timestamp(now.Add(-radarCandidateMaxAge))); err != nil {
		return report, err
	}
	users, err := store.radarReleaseUsers(ctx)
	if err != nil {
		return report, err
	}
	for _, user := range users {
		if report.Released >= maximumRadarReleasesPerPass {
			break
		}
		report.ProfilesChecked++
		count, reason, releaseErr := store.releaseRadarForUser(
			ctx, user, now, maximumRadarReleasesPerPass-report.Released,
		)
		if releaseErr != nil {
			return report, releaseErr
		}
		report.Released += count
		switch reason {
		case radarReleaseWeekendPaused:
			report.WeekendPaused++
		case radarReleaseDailyTargetMet:
			report.DailyTargetMet++
		case radarReleaseWaitingForSlot:
			report.WaitingForSlot++
		case radarReleaseNoEligibleItems:
			report.NoEligibleContent++
		}
	}
	return report, nil
}

func (store *Store) radarReleaseUsers(ctx context.Context) ([]radarReleaseUser, error) {
	rows, err := store.database.QueryContext(ctx, `SELECT lp.user_id,lp.radar_items_per_day,
		lp.radar_weekends_enabled,lp.time_zone,(
			SELECT COUNT(*) FROM radar_candidates rc
			WHERE rc.user_id=lp.user_id AND rc.status='pending'
		)
		FROM learning_preferences lp
		JOIN users u ON u.id=lp.user_id AND u.status='active'
		JOIN delivery_destinations dd ON dd.user_id=lp.user_id AND dd.channel='telegram'
			AND dd.enabled=1 AND dd.status='connected'
		WHERE lp.radar_enabled=1
		ORDER BY lp.user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []radarReleaseUser
	for rows.Next() {
		var user radarReleaseUser
		var weekends int
		if err := rows.Scan(
			&user.ID,
			&user.ItemsPerDay,
			&weekends,
			&user.TimeZone,
			&user.PendingItems,
		); err != nil {
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
) (int, string, error) {
	location, err := time.LoadLocation(user.TimeZone)
	if err != nil {
		return 0, "", fmt.Errorf("load Radar time zone: %w", err)
	}
	localNow := now.In(location)
	if !user.WeekendsEnabled && (localNow.Weekday() == time.Saturday || localNow.Weekday() == time.Sunday) {
		return 0, radarReleaseWeekendPaused, nil
	}
	if user.PendingItems == 0 {
		return 0, radarReleaseNoEligibleItems, nil
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
		return 0, radarReleaseNoEligibleItems, nil
	}

	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO radar_daily_usage (user_id,local_date)
		VALUES (?,?) ON CONFLICT(user_id,local_date) DO NOTHING`, user.ID, localDate); err != nil {
		return 0, "", err
	}
	if user.ItemsPerDay > 0 {
		var releasedCount int
		var lastReleased sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT released_count,last_released_at
			FROM radar_daily_usage WHERE user_id=? AND local_date=?`, user.ID, localDate).
			Scan(&releasedCount, &lastReleased); err != nil {
			return 0, "", err
		}
		if releasedCount >= user.ItemsPerDay {
			return 0, radarReleaseDailyTargetMet, tx.Commit()
		}
		if lastReleased.Valid {
			last, parseErr := parseTimestamp(lastReleased.String)
			if parseErr != nil {
				return 0, "", parseErr
			}
			if !radarReleaseSlotDue(last, localNow, user.ItemsPerDay) {
				return 0, radarReleaseWaitingForSlot, tx.Commit()
			}
		}
		limit = 1
	}
	candidatePool, err := radarCandidatePool(ctx, tx, user.ID, limit, now)
	if err != nil {
		return 0, "", err
	}
	sourceUsage, err := radarSourceUsage(
		ctx, tx, user.ID, localDayStart, localDayEnd,
	)
	if err != nil {
		return 0, "", err
	}

	candidateIDs := diverseRadarCandidates(candidatePool, sourceUsage, limit)
	if len(candidateIDs) == 0 {
		return 0, radarReleaseNoEligibleItems, tx.Commit()
	}
	if err := reserveRadarUsage(ctx, tx, user, localDate, len(candidateIDs), now); err != nil {
		return 0, "", err
	}
	arguments := make([]any, 0, len(candidateIDs)+3)
	arguments = append(arguments, timestamp(now), timestamp(now), user.ID)
	for _, candidateID := range candidateIDs {
		arguments = append(arguments, candidateID)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(candidateIDs)), ",")
	result, err := tx.ExecContext(ctx, `UPDATE radar_candidates
		SET status='qualified',released_at=?,updated_at=?
		WHERE user_id=? AND status='pending' AND id IN (`+placeholders+`)`, arguments...)
	if err != nil {
		return 0, "", err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, "", err
	}
	if changed != int64(len(candidateIDs)) {
		return 0, "", fmt.Errorf("reserve Radar candidates: %w", sql.ErrNoRows)
	}
	if err := insertRadarDeliveryJobs(ctx, tx, user.ID, candidateIDs, now); err != nil {
		return 0, "", err
	}
	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	return len(candidateIDs), radarReleaseCompleted, nil
}

func reserveRadarUsage(
	ctx context.Context,
	tx *sql.Tx,
	user radarReleaseUser,
	localDate string,
	count int,
	now time.Time,
) error {
	statement := `UPDATE radar_daily_usage
		SET released_count=released_count+?,last_released_at=?,updated_at=?
		WHERE user_id=? AND local_date=?`
	arguments := []any{count, timestamp(now), timestamp(now), user.ID, localDate}
	if user.ItemsPerDay > 0 {
		statement += " AND released_count+?<=?"
		arguments = append(arguments, count, user.ItemsPerDay)
	}
	result, err := tx.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("reserve Radar daily usage: %w", sql.ErrNoRows)
	}
	return nil
}

func insertRadarDeliveryJobs(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	candidateIDs []string,
	now time.Time,
) error {
	values := strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?),", len(candidateIDs)), ",")
	arguments := make([]any, 0, len(candidateIDs)*6)
	for _, candidateID := range candidateIDs {
		jobID, err := ids.New("job")
		if err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]string{"candidate_id": candidateID})
		if err != nil {
			return err
		}
		arguments = append(
			arguments, jobID, userID, "deliver_radar", timestamp(now),
			"radar-deliver:"+candidateID, string(payload),
		)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO job_queue
		(id,user_id,job_type,due_at,idempotency_key,payload_json) VALUES `+values+`
		ON CONFLICT(idempotency_key) DO NOTHING`, arguments...)
	return err
}

// radarReleaseSlotDue anchors finite Radar delivery to stable local-day slots.
// Comparing with the current slot boundary avoids cadence drift when the cron
// tick runs a few minutes late and prevents catch-up bursts for missed slots.
func radarReleaseSlotDue(lastReleased, localNow time.Time, itemsPerDay int) bool {
	if itemsPerDay <= 0 {
		return true
	}
	dayStart := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(),
		0, 0, 0, 0, localNow.Location(),
	)
	interval := 24 * time.Hour / time.Duration(itemsPerDay)
	elapsed := localNow.Sub(dayStart)
	currentSlot := dayStart.Add(elapsed / interval * interval)
	return lastReleased.Before(currentSlot)
}

func radarSourceUsage(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	dayStart time.Time,
	dayEnd time.Time,
) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT si.source_id,si.discovery_json
		FROM radar_candidates rc JOIN source_items si ON si.id=rc.source_item_id
		WHERE rc.user_id=? AND rc.released_at>=? AND rc.released_at<?
		ORDER BY rc.released_at,rc.id`, userID, timestamp(dayStart), timestamp(dayEnd))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	usage := map[string]int{}
	for rows.Next() {
		var sourceID, discoveryJSON string
		if err := rows.Scan(&sourceID, &discoveryJSON); err != nil {
			return nil, err
		}
		usage[radarSourceFamily(sourceID, discoveryJSON)]++
	}
	return usage, rows.Err()
}

type radarCandidatePoolQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// radarCandidatePool caps each individual source before applying the global
// score order. A high-volume feed such as arXiv therefore cannot crowd Hacker
// News, Stacker News, and lower-volume official engineering blogs out of the
// in-memory diversity pass.
func radarCandidatePool(
	ctx context.Context,
	queryer radarCandidatePoolQueryer,
	userID string,
	requestedCount int,
	now time.Time,
) ([]radarReleaseCandidate, error) {
	perSourceLimit := max(2, min(requestedCount, maximumUnlimitedReleasePass))
	rows, err := queryer.QueryContext(ctx, `WITH eligible AS (
		SELECT rc.id,si.source_id,si.normalized_url,si.discovery_json,rc.relevance_score,
			COALESCE(si.published_at,si.retrieved_at) AS freshness,
			rc.created_at,
			ROW_NUMBER() OVER (
				PARTITION BY si.source_id
				ORDER BY rc.relevance_score DESC,
					COALESCE(si.published_at,si.retrieved_at) DESC,
					rc.created_at,rc.id
			) AS source_position
		FROM radar_candidates rc
		JOIN source_items si ON si.id=rc.source_item_id
		WHERE rc.user_id=? AND rc.status='pending'
			AND rc.ranker_version=? AND rc.relevance_score>=?
			AND COALESCE(si.published_at,si.retrieved_at)>=?
	)
	SELECT id,source_id,normalized_url,discovery_json,relevance_score
	FROM eligible WHERE source_position<=?
	ORDER BY relevance_score DESC,freshness DESC,created_at,id`,
		userID, radar.RankerVersion, radar.MinimumDeliveryScore,
		timestamp(now.Add(-radarCandidateMaxAge)), perSourceLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pool []radarReleaseCandidate
	for rows.Next() {
		var candidate radarReleaseCandidate
		if err := rows.Scan(
			&candidate.ID,
			&candidate.SourceID,
			&candidate.URL,
			&candidate.DiscoveryJSON,
			&candidate.Score,
		); err != nil {
			return nil, err
		}
		if _, err := radar.CanonicalURL(candidate.URL); err != nil {
			continue
		}
		candidate.SourceFamily = radarSourceFamily(
			candidate.SourceID,
			candidate.DiscoveryJSON,
		)
		pool = append(pool, candidate)
	}
	return pool, rows.Err()
}

func radarSourceFamily(sourceID string, discoveryJSON ...string) string {
	if len(discoveryJSON) > 0 {
		var discovery []radar.SourceReference
		if json.Unmarshal([]byte(discoveryJSON[0]), &discovery) == nil {
			for _, reference := range discovery {
				lowerURL := strings.ToLower(reference.URL)
				switch {
				case strings.Contains(lowerURL, "news.ycombinator.com/"):
					return "hacker_news"
				case strings.Contains(lowerURL, "stacker.news/"):
					return "stacker_news"
				case strings.Contains(lowerURL, "lobste.rs/"):
					return "lobsters"
				case strings.Contains(lowerURL, "bsky.app/"):
					return "bluesky"
				}
			}
		}
	}
	switch {
	case sourceID == "source_hacker_news":
		return "hacker_news"
	case strings.HasPrefix(sourceID, "source_stacker_"):
		return "stacker_news"
	case strings.HasPrefix(sourceID, "source_arxiv_"):
		return "arxiv"
	case sourceID == "source_lobsters":
		return "lobsters"
	case strings.HasSuffix(sourceID, "_bluesky"):
		return "bluesky"
	case strings.HasPrefix(sourceID, "source_huggingface_"):
		return "huggingface"
	case strings.HasPrefix(sourceID, "source_cloudflare_"):
		return "cloudflare"
	case strings.HasPrefix(sourceID, "source_google_") || sourceID == "source_deepmind_blog":
		return "google"
	case strings.HasPrefix(sourceID, "source_github_"):
		return "github"
	case strings.HasPrefix(sourceID, "source_meta_"):
		return "meta"
	case strings.HasPrefix(sourceID, "source_microsoft_") ||
		sourceID == "source_azure_updates" ||
		sourceID == "source_msrc_blog" ||
		sourceID == "source_typescript_blog" ||
		sourceID == "source_dotnet_blog":
		return "microsoft"
	default:
		return sourceID
	}
}

func diverseRadarCandidates(pool []radarReleaseCandidate, historicalUsage map[string]int, limit int) []string {
	if limit <= 0 {
		return nil
	}
	ranked := append([]radarReleaseCandidate(nil), pool...)
	for index := range ranked {
		if ranked[index].SourceFamily == "" {
			ranked[index].SourceFamily = radarSourceFamily(
				ranked[index].SourceID,
				ranked[index].DiscoveryJSON,
			)
		}
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		leftUsage := historicalUsage[ranked[left].SourceFamily]
		rightUsage := historicalUsage[ranked[right].SourceFamily]
		if leftUsage != rightUsage {
			return leftUsage < rightUsage
		}
		return ranked[left].Score > ranked[right].Score
	})

	familyOrder := make([]string, 0, len(ranked))
	byFamily := make(map[string][]radarReleaseCandidate)
	for _, candidate := range ranked {
		family := candidate.SourceFamily
		if len(byFamily[family]) == 0 {
			familyOrder = append(familyOrder, family)
		}
		byFamily[family] = append(byFamily[family], candidate)
	}

	selected := make([]string, 0, min(limit, len(pool)))
	for round := 0; len(selected) < limit; round++ {
		added := false
		for _, family := range familyOrder {
			candidates := byFamily[family]
			if round >= len(candidates) {
				continue
			}
			selected = append(selected, candidates[round].ID)
			added = true
			if len(selected) == limit {
				return selected
			}
		}
		if !added {
			break
		}
	}
	return selected
}
