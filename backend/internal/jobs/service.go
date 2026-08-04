package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
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
)

func New(dataStore *store.Store, providerRouter *providers.Router, telegramClient *telegram.Client, publisher *qstash.Publisher, alertService *alerts.Service, appOrigin string) *Service {
	sourceClient := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 4 {
			return errors.New("too many source redirects")
		}
		return validateSourceURL(request.URL)
	}}
	return &Service{store: dataStore, providers: providerRouter, telegram: telegramClient, publisher: publisher, alerts: alertService, appOrigin: strings.TrimRight(appOrigin, "/"), http: sourceClient, now: time.Now}
}

func (service *Service) Tick(ctx context.Context) error {
	now := service.now()
	if err := service.store.RecoverJobs(ctx, now); err != nil {
		return err
	}
	occurrences, err := service.store.DueOccurrences(ctx, now, 6*time.Minute)
	if err != nil {
		return err
	}
	for _, occurrence := range occurrences {
		_, err := service.store.EnqueueJob(ctx, occurrence.UserID, "", "generate_lesson", occurrence.At, occurrence.Key, map[string]string{"scheduled_at": occurrence.At.Format(time.RFC3339)})
		if err != nil {
			return err
		}
	}
	if err := service.store.EnqueueSourcePolls(ctx, now); err != nil {
		return err
	}
	return service.dispatchNextDueJob(ctx)
}

func (service *Service) dispatchNextDueJob(ctx context.Context) error {
	queued, err := service.store.PublishableJobs(ctx, service.now(), publishableJobsPerTick)
	if err != nil {
		return err
	}
	if len(queued) == 0 {
		return nil
	}
	if service.publisher == nil {
		return errors.New("QStash publisher is not configured")
	}
	job := queued[0]
	destination := service.appOrigin + "/api/jobs/run"
	return service.publisher.Publish(ctx, destination, dispatchDeduplicationID(job.ID), map[string]string{"job_id": job.ID})
}

func (service *Service) continueQueue(ctx context.Context) {
	if err := service.dispatchNextDueJob(ctx); err != nil {
		slog.WarnContext(ctx, "next chronological job dispatch failed", "error", err)
	}
}

func dispatchDeduplicationID(jobID string) string {
	return "dispatch-" + jobID
}

func (service *Service) Run(ctx context.Context, jobID, workerID string) error {
	now := service.now()
	job, err := service.store.LeaseJob(ctx, jobID, workerID, now)
	if errors.Is(err, store.ErrJobNotLeasable) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lease job: %w", err)
	}
	if job.State != "leased" {
		return fmt.Errorf("lease job returned unexpected state %q", job.State)
	}
	executionContext, cancelExecution := jobExecutionContext(ctx)
	err = service.execute(executionContext, job)
	cancelExecution()
	if err == nil {
		if err := service.store.CompleteJob(ctx, job.ID, service.now()); err != nil {
			return err
		}
		service.continueQueue(ctx)
		return nil
	}
	quota := errors.Is(err, providers.ErrFreeQuotaExhausted)
	code := "job_failed"
	if quota {
		code = "ai_quota_exhausted"
	}
	if failErr := service.store.FailJob(ctx, job, code, quota, service.now()); failErr != nil {
		return errors.Join(err, failErr)
	}
	if quota {
		service.notifyQuota(ctx, job)
	}
	service.continueQueue(ctx)
	return fmt.Errorf("execute %s job: %w", job.Type, err)
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
	if job.AssignmentID != "" {
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
		assignmentID, err := service.store.AssignCachedLesson(ctx, plan, cachedID, service.now())
		if err != nil {
			return err
		}
		return service.enqueueLessonDelivery(ctx, job, assignmentID)
	}
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
	warning := generated.Warning
	if generated.VerificationState == "unverified_warning" && warning == "" {
		warning = "Some claims could not be fully verified from the supplied sources. Treat current or changing details as unverified."
	}
	parts := telegram.ChunkHTML(content.RenderSections(generated.Draft), warning)
	_, assignmentID, err := service.store.SaveGeneratedLesson(ctx, plan, generated, parts, cacheKey, service.now())
	if err != nil {
		return err
	}
	_ = service.store.RecordProviderRun(ctx, job, generated.Provider, generated.Model, generated.RequestID, requestKind(job, useOpenAI), "succeeded", "", generated.InputTokens, generated.OutputTokens, started, service.now())
	return service.enqueueLessonDelivery(ctx, job, assignmentID)
}

func (service *Service) deliverLesson(ctx context.Context, job store.Job) error {
	assignmentID := job.AssignmentID
	if assignmentID == "" {
		var payload map[string]string
		if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			return err
		}
		assignmentID = payload["assignment_id"]
	}
	bundle, err := service.store.PrepareDelivery(ctx, job.UserID, assignmentID, job.ID, service.now())
	if err != nil {
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
			options.Buttons = [][]telegram.Button{{{Text: "Read", Data: "read:" + assignmentID}, {Text: "Skip", Data: "skip:" + assignmentID}}}
		}
		messageID, err := service.telegram.SendMessage(ctx, bundle.ChatID, part.Text, options)
		if err != nil {
			_ = service.store.FailDelivery(ctx, bundle.ID, "telegram_send_failed", service.now())
			return err
		}
		if err := service.store.CompleteDeliveryPart(ctx, part.ID, messageID, service.now()); err != nil {
			return err
		}
	}
	return service.store.CompleteDelivery(ctx, bundle.ID, assignmentID, service.now())
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return err
	}
	if err := validateSourceURL(request.URL); err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Coreloop/1.0 (+"+service.appOrigin+")")
	if source.ETag != "" {
		request.Header.Set("If-None-Match", source.ETag)
	}
	if source.LastModified != "" {
		request.Header.Set("If-Modified-Since", source.LastModified)
	}
	response, err := service.http.Do(request)
	if err != nil {
		_ = service.store.SourcePollFailed(ctx, source.ID, service.now())
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		_, err := service.store.SaveSourceItems(ctx, source, nil, source.ETag, source.LastModified, service.now())
		return err
	}
	if response.StatusCode != http.StatusOK {
		_ = service.store.SourcePollFailed(ctx, source.ID, service.now())
		return fmt.Errorf("source returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	items, err := radar.ParseFeed(body)
	if err != nil {
		return err
	}
	if len(items) > 50 {
		items = items[:50]
	}
	itemIDs, err := service.store.SaveSourceItems(ctx, source, items, response.Header.Get("ETag"), response.Header.Get("Last-Modified"), service.now())
	if err != nil {
		return err
	}
	if len(itemIDs) == 0 {
		return nil
	}
	batchKey := job.ID
	_, err = service.store.EnqueueJob(ctx, "", "", "rank_radar", service.now(), "rank-batch:"+batchKey+":"+radar.RankerVersion, map[string]any{"source_item_ids": itemIDs})
	return err
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
	for _, itemID := range itemIDs {
		candidates, err := service.store.RankSourceItem(ctx, itemID, service.now())
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			_, err := service.store.EnqueueJob(ctx, candidate.UserID, "", "deliver_radar", service.now(), "radar-deliver:"+candidate.ID, map[string]string{"candidate_id": candidate.ID})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (service *Service) deliverRadar(ctx context.Context, job store.Job) error {
	var payload map[string]string
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return err
	}
	candidate, err := service.store.RadarCandidate(ctx, payload["candidate_id"])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	chatID, err := service.store.Destination(ctx, candidate.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.store.CompleteRadar(ctx, candidate.UserID, candidate.ID, "rejected", service.now())
		}
		return err
	}
	summary := candidate.Summary
	if len([]rune(summary)) > 900 {
		summary = string([]rune(summary)[:900]) + "…"
	}
	message := "<b>Radar · " + html.EscapeString(candidate.Publisher) + "</b>\n\n<b>" + html.EscapeString(candidate.Title) + "</b>\n" + html.EscapeString(summary) + "\n\n<a href=\"" + html.EscapeString(candidate.URL) + "\">Read the official source</a>"
	_, err = service.telegram.SendMessage(ctx, chatID, message, telegram.MessageOptions{Buttons: [][]telegram.Button{{{Text: "Skip", Data: "radar_skip:" + candidate.ID}}}})
	if err != nil {
		if telegram.IsChatUnavailable(err) {
			disconnectErr := service.store.DisconnectDestination(ctx, candidate.UserID, service.now())
			rejectErr := service.store.CompleteRadar(ctx, candidate.UserID, candidate.ID, "rejected", service.now())
			return errors.Join(disconnectErr, rejectErr)
		}
		return err
	}
	return service.store.CompleteRadar(ctx, candidate.UserID, candidate.ID, "delivered", service.now())
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
	job, err := service.store.Job(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Type != "generate_lesson" || job.State != "blocked_quota" {
		return errors.New("job is not a quota-blocked lesson generation job")
	}
	executionContext, cancelExecution := jobExecutionContext(ctx)
	err = service.generateLesson(executionContext, job, true)
	cancelExecution()
	if err != nil {
		return err
	}
	return service.store.CompleteJob(ctx, job.ID, service.now())
}

func validateSourceURL(value *url.URL) error {
	if value.Scheme != "https" || value.Host == "" {
		return errors.New("source URL must use HTTPS")
	}
	host := strings.ToLower(value.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return errors.New("local source hosts are not allowed")
	}
	if address := net.ParseIP(host); address != nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast()) {
		return errors.New("private source addresses are not allowed")
	}
	return nil
}
