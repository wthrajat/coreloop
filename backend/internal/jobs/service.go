package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"coreloop/backend/internal/alerts"
	"coreloop/backend/internal/content"
	"coreloop/backend/internal/providers"
	"coreloop/backend/internal/qstash"
	"coreloop/backend/internal/radar"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

type Service struct {
	store     *store.Store
	providers *providers.Router
	telegram  *telegram.Client
	publisher *qstash.Publisher
	alerts    *alerts.Service
	appOrigin string
	http      *http.Client
	now       func() time.Time
}

const (
	publishableJobsPerTick     = 1
	defaultJobExecutionTimeout = 45 * time.Second
	jobFinalizationReserve     = 8 * time.Second
	maximumRadarRankBatchSize  = 5
)

func New(dataStore *store.Store, providerRouter *providers.Router, telegramClient *telegram.Client, publisher *qstash.Publisher, alertService *alerts.Service, appOrigin string) *Service {
	sourceClient := &http.Client{Timeout: 20 * time.Second, Transport: sourceTransport(), CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 4 {
			return errors.New("too many source redirects")
		}
		return validateSourceURL(request.URL)
	}}
	return &Service{store: dataStore, providers: providerRouter, telegram: telegramClient, publisher: publisher, alerts: alertService, appOrigin: strings.TrimRight(appOrigin, "/"), http: sourceClient, now: time.Now}
}

func sourceTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse source address: %w", err)
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve source host: %w", err)
		}
		var failures []error
		for _, resolved := range addresses {
			if !isPublicSourceIP(resolved.IP) {
				failures = append(failures, fmt.Errorf("resolved address %s is not public", resolved.IP))
				continue
			}
			connection, err := dialer.DialContext(
				ctx, network, net.JoinHostPort(resolved.IP.String(), port),
			)
			if err == nil {
				return connection, nil
			}
			failures = append(failures, err)
		}
		if len(failures) == 0 {
			return nil, errors.New("source host resolved to no addresses")
		}
		return nil, fmt.Errorf("connect to public source: %w", errors.Join(failures...))
	}
	return transport
}

func (service *Service) Tick(ctx context.Context) error {
	now := service.now()
	slog.InfoContext(ctx, "job tick started", "tick_at", now.UTC())
	if err := service.store.RecoverJobs(ctx, now); err != nil {
		return err
	}
	if err := service.store.PruneSecurityState(ctx, now); err != nil {
		slog.WarnContext(ctx, "security state pruning failed", "error", err)
	}
	occurrences, err := service.store.DueOccurrences(ctx, now, 6*time.Minute)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "job tick evaluated lesson schedules", "due_occurrences", len(occurrences))
	for _, occurrence := range occurrences {
		_, err := service.store.EnqueueJob(ctx, occurrence.UserID, "", "generate_lesson", occurrence.At, occurrence.Key, map[string]string{"scheduled_at": occurrence.At.Format(time.RFC3339)})
		if err != nil {
			return err
		}
	}
	if err := service.store.EnqueueSourcePolls(ctx, now); err != nil {
		return err
	}
	radarRelease, err := service.store.ReleaseRadarCandidates(ctx, now)
	if err != nil {
		return err
	}
	logRadarRelease(ctx, "Radar release evaluated", radarRelease)
	service.logQueueSummary(ctx)
	return service.dispatchNextDueJob(ctx)
}

func logRadarRelease(
	ctx context.Context,
	message string,
	report store.RadarReleaseReport,
) {
	slog.InfoContext(
		ctx,
		message,
		"profiles_checked", report.ProfilesChecked,
		"released_candidates", report.Released,
		"waiting_for_slot", report.WaitingForSlot,
		"daily_target_met", report.DailyTargetMet,
		"weekend_paused", report.WeekendPaused,
		"no_eligible_content", report.NoEligibleContent,
	)
}

func (service *Service) logQueueSummary(ctx context.Context) {
	summary, err := service.store.JobQueueSummary(ctx)
	if err != nil {
		slog.WarnContext(ctx, "durable queue summary unavailable", "error", err)
		return
	}
	if len(summary) == 0 {
		slog.InfoContext(ctx, "durable queue is empty")
		return
	}
	for _, item := range summary {
		slog.InfoContext(
			ctx,
			"durable queue state",
			"job_type", item.Type,
			"job_state", item.State,
			"count", item.Count,
			"oldest_due_at", item.OldestDueAt.UTC(),
		)
	}
}

func (service *Service) dispatchNextDueJob(ctx context.Context) error {
	now := service.now()
	queued, err := service.store.PublishableJobs(ctx, now, publishableJobsPerTick)
	if err != nil {
		return err
	}
	if len(queued) == 0 {
		slog.InfoContext(ctx, "durable queue has no due jobs")
		return nil
	}
	if service.publisher == nil {
		return errors.New("QStash publisher is not configured")
	}
	job := queued[0]
	destination := service.appOrigin + "/api/jobs/run"
	deduplicationID := scheduledDispatchDeduplicationID(
		job.ID,
		job.AttemptCount,
		now,
	)
	slog.InfoContext(
		ctx,
		"job dispatch requested",
		"job_id", job.ID,
		"job_type", job.Type,
		"sequence", job.Sequence,
		"attempt", job.AttemptCount,
		"due_at", job.DueAt.UTC(),
	)
	if err := service.publisher.Publish(ctx, destination, deduplicationID, map[string]string{"job_id": job.ID}); err != nil {
		return err
	}
	slog.InfoContext(
		ctx,
		"job dispatch accepted",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.AttemptCount,
	)
	return nil
}

func (service *Service) continueQueue(ctx context.Context) error {
	if err := service.dispatchNextDueJob(ctx); err != nil {
		return fmt.Errorf("dispatch next chronological job: %w", err)
	}
	return nil
}

func dispatchDeduplicationID(jobID string, attempt int) string {
	return fmt.Sprintf("dispatch-%s-%d", jobID, attempt)
}

// scheduledDispatchDeduplicationID suppresses duplicate publishes within one
// scheduler window but permits the next tick to wake the same queued attempt.
// This heals the queue when an earlier QStash message was acknowledged before
// the job obtained its lease; an attempt-only key would remain deduplicated and
// strand every newer chronological job behind it.
func scheduledDispatchDeduplicationID(
	jobID string,
	attempt int,
	now time.Time,
) string {
	window := now.UTC().Truncate(10 * time.Minute).Format("20060102T1504")
	return fmt.Sprintf("dispatch-%s-%d-%s", jobID, attempt, window)
}

func (service *Service) Run(ctx context.Context, jobID, workerID string) error {
	now := service.now()
	slog.InfoContext(ctx, "job wake received", "job_id", jobID)
	job, err := service.store.LeaseJob(ctx, jobID, workerID, now)
	if errors.Is(err, store.ErrJobNotLeasable) {
		slog.InfoContext(
			ctx,
			"job wake acknowledged without lease",
			"job_id", jobID,
			"job_state", job.State,
		)
		if job.State == "queued" {
			exhausted, finalizeError := service.store.FailExhaustedJob(ctx, jobID, service.now())
			if finalizeError != nil {
				return fmt.Errorf("finalize exhausted queued job: %w", finalizeError)
			}
			if exhausted {
				slog.WarnContext(ctx, "exhausted queued job finalized", "job_id", jobID)
				return service.continueQueue(ctx)
			}
		}
		if job.State == "completed" || job.State == "failed" || job.State == "blocked_quota" {
			return service.continueQueue(ctx)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("lease job: %w", err)
	}
	if job.State != "leased" {
		return fmt.Errorf("lease job returned unexpected state %q", job.State)
	}
	slog.InfoContext(
		ctx,
		"job lease acquired",
		"job_id", job.ID,
		"job_type", job.Type,
		"sequence", job.Sequence,
		"attempt", job.AttemptCount,
		"max_attempts", job.MaxAttempts,
	)
	executionStarted := service.now()
	executionContext, cancelExecution := jobExecutionContext(ctx)
	err = service.execute(executionContext, job)
	cancelExecution()
	if err == nil {
		if err := service.store.CompleteJob(ctx, job.ID, workerID, service.now()); err != nil {
			return err
		}
		slog.InfoContext(
			ctx,
			"job completed",
			"job_id", job.ID,
			"job_type", job.Type,
			"attempt", job.AttemptCount,
			"execution_ms", service.now().Sub(executionStarted).Milliseconds(),
		)
		return service.continueQueue(ctx)
	}
	quota := errors.Is(err, providers.ErrFreeQuotaExhausted)
	code := "job_failed"
	if quota {
		code = "ai_quota_exhausted"
	}
	if failErr := service.store.FailJob(ctx, job, workerID, code, quota, service.now()); failErr != nil {
		return errors.Join(err, failErr)
	}
	slog.WarnContext(
		ctx,
		"job failure persisted",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.AttemptCount,
		"next_state", failedJobState(job, quota),
		"error_code", code,
		"execution_ms", service.now().Sub(executionStarted).Milliseconds(),
	)
	if quota {
		service.notifyQuota(ctx, job)
	}
	continuationError := service.continueQueue(ctx)
	return errors.Join(fmt.Errorf("execute %s job: %w", job.Type, err), continuationError)
}

func failedJobState(job store.Job, quota bool) string {
	if quota {
		return "blocked_quota"
	}
	if job.AttemptCount >= job.MaxAttempts {
		return "failed"
	}
	return "queued"
}

func jobExecutionContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parentDeadline, ok := parent.Deadline(); ok {
		return context.WithDeadline(parent, parentDeadline.Add(-jobFinalizationReserve))
	}
	return context.WithTimeout(parent, defaultJobExecutionTimeout)
}

func (service *Service) execute(ctx context.Context, job store.Job) error {
	switch job.Type {
	case "generate_lesson":
		return service.generateLesson(ctx, job, false)
	case "deliver_lesson":
		return service.deliverLesson(ctx, job)
	case "ingest_source":
		return service.ingestSource(ctx, job)
	case "rank_radar":
		return service.rankRadar(ctx, job)
	case "deliver_radar":
		return service.deliverRadar(ctx, job)
	case "recover":
		return service.store.RecoverJobs(ctx, service.now())
	default:
		return fmt.Errorf("unknown job type %q", job.Type)
	}
}

func (service *Service) generateLesson(ctx context.Context, job store.Job, useOpenAI bool) error {
	slog.InfoContext(ctx, "lesson generation started", "job_id", job.ID, "attempt", job.AttemptCount)
	if job.AssignmentID != "" {
		slog.InfoContext(ctx, "lesson generation reused linked assignment", "job_id", job.ID)
		return service.enqueueLessonDelivery(ctx, job, job.AssignmentID)
	}
	plan, err := service.store.PlanNextLesson(ctx, job.UserID, service.now())
	if err != nil {
		return err
	}
	lessonContext := plan.Context()
	cacheKey, err := content.CacheKey(lessonContext)
	if err != nil {
		return err
	}
	_, cachedID, _, err := service.store.CachedLesson(ctx, cacheKey)
	if err == nil {
		slog.InfoContext(ctx, "lesson cache hit", "job_id", job.ID)
		assignmentID, err := service.store.AssignCachedLesson(ctx, plan, cachedID, service.now())
		if err != nil {
			return err
		}
		return service.enqueueLessonDelivery(ctx, job, assignmentID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("look up cached lesson: %w", err)
	}
	slog.InfoContext(ctx, "lesson cache miss", "job_id", job.ID)
	started := service.now()
	var generated content.Generated
	if useOpenAI {
		generated, err = service.providers.GenerateWithOpenAI(ctx, lessonContext)
	} else {
		generated, err = service.providers.Generate(ctx, lessonContext)
	}
	if err != nil {
		provider := "groq"
		var providerError *providers.Error
		if errors.As(err, &providerError) {
			provider = providerError.Provider
		}
		if useOpenAI {
			provider = "openai"
		}
		state := "transient_error"
		if errors.Is(err, providers.ErrFreeQuotaExhausted) {
			state = "quota_exhausted"
		}
		_ = service.store.RecordProviderRun(ctx, job, provider, "", "", requestKind(job, useOpenAI), state, "generation_failed", 0, 0, started, service.now())
		return err
	}
	if !content.DeliveryReady(generated.Draft, lessonContext.Minutes) {
		return store.ErrIncompleteLesson
	}
	warning := generated.Warning
	if generated.VerificationState == "unverified_warning" && warning == "" {
		warning = "Some claims could not be fully verified from the supplied sources. Treat current or changing details as unverified."
	}
	parts := telegram.ChunkHTML(content.RenderSections(generated.Draft), warning)
	slog.InfoContext(
		ctx,
		"lesson provider generation succeeded",
		"job_id", job.ID,
		"provider", generated.Provider,
		"model", generated.Model,
		"input_tokens", generated.InputTokens,
		"output_tokens", generated.OutputTokens,
		"parts", len(parts),
		"generation_ms", service.now().Sub(started).Milliseconds(),
	)
	_, assignmentID, err := service.store.SaveGeneratedLesson(ctx, plan, generated, parts, cacheKey, service.now())
	if err != nil {
		return err
	}
	_ = service.store.RecordProviderRun(ctx, job, generated.Provider, generated.Model, generated.RequestID, requestKind(job, useOpenAI), "succeeded", "", generated.InputTokens, generated.OutputTokens, started, service.now())
	return service.enqueueLessonDelivery(ctx, job, assignmentID)
}

func (service *Service) deliverLesson(ctx context.Context, job store.Job) error {
	slog.InfoContext(ctx, "lesson delivery started", "job_id", job.ID, "attempt", job.AttemptCount)
	assignmentID := job.AssignmentID
	if assignmentID == "" {
		var payload map[string]string
		if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			return err
		}
		assignmentID = payload["assignment_id"]
	}
	if assignmentID == "" {
		return errors.New("lesson delivery assignment is missing")
	}
	bundle, err := service.store.PrepareDelivery(ctx, job.UserID, assignmentID, job.ID, service.now())
	if err != nil {
		return err
	}
	if bundle.State == "delivered" {
		slog.InfoContext(ctx, "lesson delivery already completed", "job_id", job.ID)
		return nil
	}
	recallQuestion, err := service.store.AssignmentRecallQuestion(
		ctx, job.UserID, assignmentID,
	)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "lesson delivery bundle prepared", "job_id", job.ID, "parts", len(bundle.Parts))
	for index, part := range bundle.Parts {
		if part.State == "delivered" {
			continue
		}
		options := telegram.MessageOptions{}
		if index == len(bundle.Parts)-1 {
			options.Buttons = [][]telegram.Button{{{Text: "Read", Data: "read:" + assignmentID}, {Text: "Skip", Data: "skip:" + assignmentID}}}
		}
		messageText, err := lessonDeliveryText(
			part.Text, recallQuestion, index == 0,
		)
		if err != nil {
			return err
		}
		messageID, err := service.telegram.SendMessage(ctx, bundle.ChatID, messageText, options)
		if err != nil {
			_ = service.store.FailDelivery(ctx, bundle.ID, "telegram_send_failed", service.now())
			return err
		}
		if err := service.store.CompleteDeliveryPart(ctx, part.ID, messageID, service.now()); err != nil {
			return err
		}
		slog.InfoContext(
			ctx,
			"lesson delivery part completed",
			"job_id", job.ID,
			"part", index+1,
			"parts", len(bundle.Parts),
		)
	}
	if err := service.store.CompleteDelivery(ctx, bundle.ID, assignmentID, service.now()); err != nil {
		return err
	}
	slog.InfoContext(ctx, "lesson delivery completed", "job_id", job.ID, "parts", len(bundle.Parts))
	return nil
}

func lessonDeliveryText(rendered, recallQuestion string, firstPart bool) (string, error) {
	message := content.SanitizeRenderedSourceLinks(rendered)
	if !firstPart || strings.TrimSpace(recallQuestion) == "" {
		return message, nil
	}
	recall := "<b>Quick recall from an earlier lesson</b>\n" +
		html.EscapeString(strings.TrimSpace(recallQuestion)) + "\n\n"
	if len([]rune(recall+message)) > 4096 {
		return "", errors.New("recall question does not fit in the first Telegram lesson part")
	}
	return recall + message, nil
}

func (service *Service) ingestSource(ctx context.Context, job store.Job) error {
	var payload map[string]string
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return err
	}
	source, err := service.store.Source(ctx, payload["source_id"])
	if err != nil {
		return err
	}
	result, err := service.fetchSource(ctx, source)
	if err != nil {
		_ = service.store.SourcePollFailed(ctx, source.ID, service.now())
		return err
	}
	if result.NotModified {
		_, err := service.store.SaveSourceItems(ctx, source, nil, result.ETag, result.LastModified, service.now())
		if err == nil {
			slog.InfoContext(ctx, "source poll not modified", "source_id", source.ID, "publisher", source.Publisher)
		}
		return err
	}
	itemIDs, err := service.store.SaveSourceItems(
		ctx, source, result.Items, result.ETag, result.LastModified, service.now(),
	)
	if err != nil {
		_ = service.store.SourcePollFailed(ctx, source.ID, service.now())
		return err
	}
	if len(itemIDs) == 0 {
		slog.InfoContext(ctx, "source poll completed", "source_id", source.ID,
			"publisher", source.Publisher, "fetched_items", len(result.Items), "changed_items", 0)
		return nil
	}
	slog.InfoContext(ctx, "source poll completed", "source_id", source.ID,
		"publisher", source.Publisher, "fetched_items", len(result.Items), "changed_items", len(itemIDs))
	for batchIndex, batch := range splitRadarItemIDs(itemIDs, maximumRadarRankBatchSize) {
		idempotencyKey := fmt.Sprintf(
			"rank-batch:%s:%s:%d", job.ID, radar.RankerVersion, batchIndex,
		)
		if _, err := service.store.EnqueueJob(
			ctx, "", "", "rank_radar", service.now(), idempotencyKey,
			map[string]any{"source_item_ids": batch},
		); err != nil {
			return err
		}
	}
	return nil
}

func splitRadarItemIDs(itemIDs []string, batchSize int) [][]string {
	if batchSize <= 0 {
		return nil
	}
	batches := make([][]string, 0, (len(itemIDs)+batchSize-1)/batchSize)
	for start := 0; start < len(itemIDs); start += batchSize {
		end := min(start+batchSize, len(itemIDs))
		batches = append(batches, itemIDs[start:end])
	}
	return batches
}

func (service *Service) rankRadar(ctx context.Context, job store.Job) error {
	var payload struct {
		SourceItemID  string   `json:"source_item_id"`
		SourceItemIDs []string `json:"source_item_ids"`
	}
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return err
	}
	itemIDs := payload.SourceItemIDs
	if payload.SourceItemID != "" {
		itemIDs = append(itemIDs, payload.SourceItemID)
	}
	candidateCount, err := service.store.RankSourceItems(ctx, itemIDs, service.now())
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "Radar ranking completed", "job_id", job.ID,
		"source_items", len(itemIDs), "pending_candidates", candidateCount)
	if candidateCount > 0 {
		releaseReport, releaseErr := service.store.ReleaseRadarCandidates(
			ctx,
			service.now(),
		)
		if releaseErr != nil {
			return fmt.Errorf("release newly ranked Radar candidates: %w", releaseErr)
		}
		logRadarRelease(ctx, "Radar post-ranking release evaluated", releaseReport)
	}
	return nil
}

func (service *Service) deliverRadar(ctx context.Context, job store.Job) error {
	slog.InfoContext(ctx, "Radar delivery started", "job_id", job.ID, "attempt", job.AttemptCount)
	var payload radarJobMetadata
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return err
	}
	candidate, err := service.store.RadarCandidate(ctx, payload.CandidateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	now := service.now()
	rejectionReason := radarDeliveryRejectionReason(candidate, now)
	if rejectionReason != "" {
		slog.InfoContext(ctx, "Radar delivery suppressed", "job_id", job.ID,
			"candidate_id", candidate.ID, "reason", rejectionReason,
			"published_at", candidate.PublishedAt.UTC())
		return service.store.RejectRadar(
			ctx, candidate.UserID, candidate.ID, rejectionReason, now,
		)
	}
	sourceName := radarSourceName(candidate)
	summary, whyItMatters := service.radarBriefingContent(ctx, candidate)
	briefing, err := radar.RenderCompactBriefing(radar.BriefingInput{
		Category: radar.Category(candidate.Category), Title: candidate.Title,
		Summary: summary, WhyItMatters: whyItMatters,
		Source:        radar.SourceReference{Name: sourceName, URL: candidate.URL},
		DiscoveredVia: candidate.Discovery,
	}, telegram.SafeChunkCharacters)
	if err != nil {
		return fmt.Errorf("render deterministic Radar briefing: %w", err)
	}
	parts := []string{html.EscapeString(briefing)}
	bundle, err := service.store.PrepareRadarDelivery(
		ctx, candidate.UserID, candidate.ID, job.ID, parts, service.now(),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.store.CompleteRadar(ctx, candidate.UserID, candidate.ID, "rejected", service.now())
		}
		return err
	}
	if bundle.State == "delivered" {
		return nil
	}
	for index, part := range bundle.Parts {
		if part.State == "delivered" {
			continue
		}
		options := telegram.MessageOptions{}
		if index == len(bundle.Parts)-1 {
			options.Buttons = [][]telegram.Button{{
				{Text: "Open source", URL: candidate.URL},
				{Text: "Skip", Data: "radar_skip:" + candidate.ID},
			}}
		}
		messageID, sendErr := service.telegram.SendMessage(ctx, bundle.ChatID, part.Text, options)
		if sendErr != nil {
			_ = service.store.FailRadarDelivery(ctx, bundle.ID, "telegram_send_failed", service.now())
			if telegram.IsChatUnavailable(sendErr) {
				disconnectErr := service.store.DisconnectDestination(ctx, candidate.UserID, service.now())
				rejectErr := service.store.CompleteRadar(ctx, candidate.UserID, candidate.ID, "rejected", service.now())
				return errors.Join(disconnectErr, rejectErr)
			}
			return sendErr
		}
		if err := service.store.CompleteRadarDeliveryPart(ctx, part.ID, messageID, service.now()); err != nil {
			return err
		}
		slog.InfoContext(ctx, "Radar delivery part completed", "job_id", job.ID,
			"part", index+1, "parts", len(bundle.Parts))
	}
	if err := service.store.CompleteRadarDelivery(
		ctx, bundle.ID, candidate.UserID, candidate.ID, service.now(),
	); err != nil {
		return err
	}
	slog.InfoContext(ctx, "Radar delivery completed", "job_id", job.ID, "parts", len(bundle.Parts))
	return nil
}

func radarSourceName(candidate store.RadarCandidate) string {
	if candidate.SourceRole != "community_discovery" || len(candidate.Discovery) == 0 {
		return candidate.Publisher
	}
	parsed, err := url.Parse(candidate.URL)
	if err != nil || parsed.Hostname() == "" {
		return "Original source"
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

func radarDeliveryRejectionReason(candidate store.RadarCandidate, now time.Time) string {
	switch {
	case !deliverableRadarURL(candidate.URL):
		return "invalid_source_url"
	case candidate.RankerVersion != radar.RankerVersion:
		return "superseded_ranking_policy"
	case candidate.Score < radar.MinimumDeliveryScore:
		return "below_editorial_threshold"
	case candidate.PublishedAt.Before(now.Add(-radar.MaximumItemAge)):
		return "outside_freshness_window"
	default:
		return ""
	}
}

func deliverableRadarURL(rawURL string) bool {
	canonical, err := radar.CanonicalURL(rawURL)
	return err == nil && canonical != ""
}

func radarDeveloperContext(category string) string {
	switch radar.Category(category) {
	case radar.CategorySecurity:
		return "Check whether affected software, dependencies, or services appear in systems you operate, then review the source for impact and remediation details."
	case radar.CategoryRelease:
		return "If you use this project, review compatibility, migration, and deployment notes before adopting the release."
	case radar.CategoryResearch:
		return "This may indicate a useful new technique or result, but research claims still need methodology and real-world validation before production use."
	case radar.CategoryPricing:
		return "This may change architecture or provider decisions where operating cost is an important constraint."
	case radar.CategoryFunding:
		return "Funding can affect a company's hiring, product roadmap, and ability to maintain or expand its technology."
	case radar.CategoryPartnership:
		return "The practical value depends on what the integration makes available to developers and whether it changes existing workflows."
	case radar.CategoryIndustry:
		return "This may influence technology adoption, regulation, company strategy, or the tools available to engineering teams."
	case radar.CategoryDiscussion:
		return "Community discussion can reveal operational experience and trade-offs that do not appear in official documentation."
	case radar.CategoryEngineering:
		return "The implementation details may contain reusable architecture, reliability, performance, or migration lessons."
	default:
		return "Review the concrete capabilities, availability, and limitations to decide whether this changes tools or systems relevant to you."
	}
}

func (service *Service) notifyQuota(ctx context.Context, job store.Job) {
	if job.UserID != "" {
		notificationID, created, _ := service.store.NotificationOnce(ctx, job.UserID, "quota_exhausted", "quota:"+job.UserID+":"+service.now().UTC().Format("2006-01-02"))
		if created {
			if chatID, err := service.store.Destination(ctx, job.UserID); err == nil {
				_, err = service.telegram.SendMessage(ctx, chatID, "AI quota is currently exhausted. Your lesson is safely queued and delivery will resume when a free provider is available.", telegram.MessageOptions{})
				code := ""
				if err != nil {
					code = "telegram_send_failed"
				}
				_ = service.store.CompleteNotification(ctx, notificationID, service.now(), code)
			}
		}
	}
	if service.alerts != nil {
		notificationID, created, _ := service.store.NotificationOnce(ctx, "", "admin_alert", "admin-quota:"+service.now().UTC().Format("2006-01-02T15"))
		if !created {
			return
		}
		if err := service.alerts.Send(ctx, "Coreloop: free AI quota exhausted", "A scheduled lesson is blocked because all configured free AI providers are unavailable or out of quota. No lesson content or user profile data is included in this alert."); err != nil {
			slog.ErrorContext(ctx, "send quota owner alert", "error", err)
			_ = service.store.CompleteNotification(ctx, notificationID, service.now(), "smtp_failed")
		} else {
			_ = service.store.CompleteNotification(ctx, notificationID, service.now(), "")
		}
	}
}

func (service *Service) RunBlockedWithOpenAI(ctx context.Context, jobID string) error {
	owner := "manual-openai:" + jobID
	job, err := service.store.ClaimBlockedJob(ctx, jobID, owner, service.now())
	if err != nil {
		if errors.Is(err, store.ErrJobNotLeasable) {
			return errors.New("job is not available for a manual OpenAI run")
		}
		return err
	}
	executionContext, cancelExecution := jobExecutionContext(ctx)
	err = service.generateLesson(executionContext, job, true)
	cancelExecution()
	if err != nil {
		if failError := service.store.FailJob(ctx, job, owner, "manual_openai_failed", true, service.now()); failError != nil {
			return errors.Join(err, failError)
		}
		return err
	}
	return service.store.CompleteJob(ctx, job.ID, owner, service.now())
}

func validateSourceURL(value *url.URL) error {
	if value.Scheme != "https" || value.Host == "" {
		return errors.New("source URL must use HTTPS")
	}
	host := strings.ToLower(value.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return errors.New("local source hosts are not allowed")
	}
	if address := net.ParseIP(host); address != nil && !isPublicSourceIP(address) {
		return errors.New("private source addresses are not allowed")
	}
	return nil
}

func isPublicSourceIP(address net.IP) bool {
	if address == nil || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsUnspecified() {
		return false
	}
	parsed, ok := netip.AddrFromSlice(address)
	if !ok {
		return false
	}
	parsed = parsed.Unmap()
	for _, blocked := range nonPublicSourcePrefixes {
		if blocked.Contains(parsed) {
			return false
		}
	}
	return true
}

var nonPublicSourcePrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001:db8::/32"),
}
