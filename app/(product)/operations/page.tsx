"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type {
  FailedJob,
  JobFailureEvent,
  ManualLesson,
  ManualRadar,
  Operations,
  SourceHealth,
} from "@/lib/api-types";
import { formatIndiaDateTime } from "@/lib/date-time";

const ACTIVE_LESSON_STATES = new Set<ManualLesson["state"]>([
  "queued",
  "generating",
  "delivering",
]);

const ACTIVE_RADAR_STATES = new Set<ManualRadar["state"]>([
  "queued",
  "delivering",
]);

export default function OperationsPage() {
  const [operations, setOperations] = useState<Operations | null>(null);
  const [inviteURL, setInviteURL] = useState("");
  const [message, setMessage] = useState("");
  const [messageTitle, setMessageTitle] = useState("");
  const [messageIsError, setMessageIsError] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [invitePending, setInvitePending] = useState(false);
  const [copyPending, setCopyPending] = useState(false);
  const [openAIPendingJob, setOpenAIPendingJob] = useState("");
  const [lesson, setLesson] = useState<ManualLesson | null>(null);
  const [lessonStarting, setLessonStarting] = useState(false);
  const [lessonError, setLessonError] = useState("");
  const [radar, setRadar] = useState<ManualRadar | null>(null);
  const [radarStarting, setRadarStarting] = useState(false);
  const [radarError, setRadarError] = useState("");
  const lessonRequestID = useRef("");
  const radarRequestID = useRef("");
  const inviteRequestActive = useRef(false);
  const openAIRequestActive = useRef(false);

  const refresh = useCallback(async () => {
    const value = await api<Operations>("/operations");
    setOperations(value);
    setLoadError("");
  }, []);

  const retryLoad = useCallback(async () => {
    setLoadError("");
    try {
      await refresh();
    } catch (reason) {
      setLoadError(
        reason instanceof Error
          ? reason.message
          : "Operational state could not be loaded.",
      );
    }
  }, [refresh]);

  useEffect(() => {
    let active = true;
    api<Operations>("/operations")
      .then((value) => {
        if (active) setOperations(value);
      })
      .catch((reason: Error) => {
        if (active) setLoadError(reason.message);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    const jobID = lesson?.job_id;
    if (!jobID || !ACTIVE_LESSON_STATES.has(lesson.state)) return;
    const currentJobID = jobID;

    let active = true;
    let pollTimer: number | undefined;
    let requestController: AbortController | undefined;
    async function pollLesson() {
      try {
        requestController = new AbortController();
        const value = await api<ManualLesson>(
          `/operations/lessons/${encodeURIComponent(currentJobID)}`,
          { signal: requestController.signal, timeoutMs: 10_000 },
        );
        if (!active) return;
        setLesson(value);
        setLessonError("");
        if (!ACTIVE_LESSON_STATES.has(value.state)) {
          void refresh().catch(() => {
            if (!active) return;
            setMessageIsError(true);
            setMessageTitle("Queue summary unavailable");
            setMessage(
              "Lesson status is complete, but the queue summary could not be refreshed.",
            );
          });
          return;
        }
      } catch (error) {
        if (!active) return;
        setLessonError(
          error instanceof Error
            ? error.message
            : "Lesson status could not be refreshed.",
        );
      }
      if (active) pollTimer = window.setTimeout(() => void pollLesson(), 2000);
    }

    pollTimer = window.setTimeout(() => void pollLesson(), 800);
    return () => {
      active = false;
      requestController?.abort();
      if (pollTimer !== undefined) window.clearTimeout(pollTimer);
    };
  }, [lesson?.job_id, lesson?.state, refresh]);

  useEffect(() => {
    const batchID = radar?.batch_id;
    if (!batchID || !ACTIVE_RADAR_STATES.has(radar.state)) return;
    const currentBatchID = batchID;

    let active = true;
    let pollTimer: number | undefined;
    let requestController: AbortController | undefined;
    async function pollRadar() {
      try {
        requestController = new AbortController();
        const value = await api<ManualRadar>(
          `/operations/radar/${encodeURIComponent(currentBatchID)}`,
          { signal: requestController.signal, timeoutMs: 10_000 },
        );
        if (!active) return;
        setRadar(value);
        setRadarError("");
        if (!ACTIVE_RADAR_STATES.has(value.state)) {
          void refresh().catch(() => {
            if (!active) return;
            setMessageIsError(true);
            setMessageTitle("Queue summary unavailable");
            setMessage(
              "Radar delivery is complete, but the queue summary could not be refreshed.",
            );
          });
          return;
        }
      } catch (error) {
        if (!active) return;
        setRadarError(
          error instanceof Error
            ? error.message
            : "Radar status could not be refreshed.",
        );
      }
      if (active) pollTimer = window.setTimeout(() => void pollRadar(), 2000);
    }

    pollTimer = window.setTimeout(() => void pollRadar(), 800);
    return () => {
      active = false;
      requestController?.abort();
      if (pollTimer !== undefined) window.clearTimeout(pollTimer);
    };
  }, [radar?.batch_id, radar?.state, refresh]);

  async function sendLessonNow() {
    if (lessonStarting) return;
    const requestID = lessonRequestID.current || crypto.randomUUID();
    lessonRequestID.current = requestID;
    setLessonStarting(true);
    setLessonError("");
    try {
      const value = await api<ManualLesson>("/operations/lessons", {
        method: "POST",
        body: JSON.stringify({ request_id: requestID }),
      });
      lessonRequestID.current = "";
      setLesson(value);
    } catch (error) {
      setLessonError(
        error instanceof Error ? error.message : "Lesson creation failed.",
      );
      return;
    } finally {
      setLessonStarting(false);
    }
    try {
      await refresh();
    } catch {
      setMessageIsError(true);
      setMessageTitle("Queue summary unavailable");
      setMessage(
        "The lesson job was accepted, but the queue summary could not be refreshed.",
      );
    }
  }

  async function sendRadarNow() {
    if (radarStarting) return;
    const requestID = radarRequestID.current || crypto.randomUUID();
    radarRequestID.current = requestID;
    setRadarStarting(true);
    setRadarError("");
    try {
      const value = await api<ManualRadar>("/operations/radar", {
        method: "POST",
        body: JSON.stringify({ request_id: requestID }),
      });
      radarRequestID.current = "";
      setRadar(value);
    } catch (error) {
      setRadarError(
        error instanceof Error ? error.message : "Radar delivery failed.",
      );
      return;
    } finally {
      setRadarStarting(false);
    }
    try {
      await refresh();
    } catch {
      setMessageIsError(true);
      setMessageTitle("Queue summary unavailable");
      setMessage(
        "The Radar batch was accepted, but the queue summary could not be refreshed.",
      );
    }
  }

  async function createInvite() {
    if (inviteRequestActive.current) return;
    inviteRequestActive.current = true;
    setInvitePending(true);
    try {
      const value = await api<{ url: string }>("/invites", {
        method: "POST",
        body: JSON.stringify({ expires_hours: 168 }),
      });
      setInviteURL(value.url);
      setMessageIsError(false);
      setMessageTitle("Invite ready");
      setMessage("A single-use seven-day invite is ready.");
    } catch (error) {
      setMessageIsError(true);
      setMessageTitle("Invite creation failed");
      setMessage(
        error instanceof Error ? error.message : "Invite creation failed.",
      );
    } finally {
      inviteRequestActive.current = false;
      setInvitePending(false);
    }
  }

  async function copyInvite() {
    if (copyPending || !inviteURL) return;
    setCopyPending(true);
    try {
      await navigator.clipboard.writeText(inviteURL);
      setMessageIsError(false);
      setMessageTitle("Invite copied");
      setMessage("Invite link copied.");
    } catch (error) {
      setMessageIsError(true);
      setMessageTitle("Copy failed");
      setMessage(
        error instanceof Error
          ? error.message
          : "Invite link could not be copied.",
      );
    } finally {
      setCopyPending(false);
    }
  }

  async function runOpenAI(jobID: string) {
    if (openAIRequestActive.current) return;
    if (
      !window.confirm(
        "Run this one blocked lesson with paid OpenAI credits? This is the only paid-provider path.",
      )
    )
      return;
    openAIRequestActive.current = true;
    setOpenAIPendingJob(jobID);
    let openAICompleted = false;
    try {
      await api<void>("/operations/openai", {
        method: "POST",
        body: JSON.stringify({ job_id: jobID }),
        timeoutMs: 55_000,
      });
      openAICompleted = true;
      setMessageIsError(false);
      setMessageTitle("OpenAI run complete");
      setMessage(
        "The explicit OpenAI run completed and its delivery is queued.",
      );
      if (lesson?.job_id === jobID) {
        try {
          setLesson(
            await api<ManualLesson>(
              `/operations/lessons/${encodeURIComponent(jobID)}`,
            ),
          );
          setLessonError("");
        } catch (error) {
          setLessonError(
            error instanceof Error
              ? error.message
              : "Lesson status could not be refreshed.",
          );
        }
      }
    } catch (error) {
      setMessageIsError(true);
      setMessageTitle("OpenAI run failed");
      setMessage(error instanceof Error ? error.message : "OpenAI run failed.");
    } finally {
      openAIRequestActive.current = false;
      setOpenAIPendingJob("");
    }
    if (!openAICompleted) return;
    try {
      await refresh();
    } catch {
      setMessageIsError(true);
      setMessageTitle("Queue summary unavailable");
      setMessage(
        "The OpenAI run completed, but the queue summary could not be refreshed.",
      );
    }
  }

  if (!operations) {
    if (loadError) {
      return (
        <div className="page-stack">
          <PageHeader
            title="Operations"
            description="Owner-only queue and delivery controls."
          />
          <section className="notice notice-error" role="alert">
            <div>
              <strong>Operational state unavailable</strong>
              <p>{loadError}</p>
            </div>
            <button
              className="button button-secondary"
              onClick={() => void retryLoad()}
              type="button"
            >
              Try again
            </button>
          </section>
        </div>
      );
    }
    return (
      <div className="page-stack" role="status">
        <span aria-hidden="true" className="loading-line" />
        <p className="muted-copy">Loading operational state…</p>
      </div>
    );
  }

  return (
    <div className="page-stack">
      <PageHeader
        title="Operations"
        description="Owner-only Telegram acceptance tests, queue truth, private invitations, and the explicit paid-provider boundary."
        action={
          <button
            className="button button-secondary"
            disabled={invitePending}
            onClick={() => void createInvite()}
            type="button"
          >
            {invitePending ? "Creating invite…" : "Create invite"}
          </button>
        }
      />
      <LessonNowPanel
        lesson={lesson}
        starting={lessonStarting}
        error={lessonError}
        onSend={() => void sendLessonNow()}
      />
      <RadarNowPanel
        radar={radar}
        starting={radarStarting}
        error={radarError}
        onSend={() => void sendRadarNow()}
      />
      {message ? (
        <section
          className={`notice ${messageIsError ? "notice-error" : "notice-success"}`}
          role={messageIsError ? "alert" : "status"}
        >
          <div>
            <strong>{messageTitle}</strong>
            <p>{message}</p>
          </div>
        </section>
      ) : null}
      {inviteURL ? (
        <section className="invite-output">
          <label className="field">
            <span>Single-use invite</span>
            <input
              readOnly
              value={inviteURL}
              onFocus={(event) => event.currentTarget.select()}
            />
            <small>
              Share privately. It expires in seven days and can create only one
              new profile.
            </small>
          </label>
          <button
            className="button button-secondary"
            disabled={copyPending}
            onClick={() => void copyInvite()}
            type="button"
          >
            {copyPending ? "Copying…" : "Copy link"}
          </button>
        </section>
      ) : null}
      <section className="metric-grid">
        <Metric label="Queued" value={operations.queued} tone="neutral" />
        <Metric label="Leased" value={operations.leased} tone="ready" />
        <Metric
          label="Quota blocked"
          value={operations.blocked_quota}
          tone={operations.blocked_quota ? "error" : "ready"}
        />
        <Metric
          label="Failed"
          value={operations.failed}
          tone={operations.failed ? "attention" : "ready"}
          detailsHref={operations.failed ? "#failed-jobs" : undefined}
        />
        <Metric label="Users" value={operations.users} tone="neutral" />
        <Metric label="Sources" value={operations.sources} tone="neutral" />
      </section>
      <SourceHealthPanel sources={operations.source_health ?? []} />
      <FailedJobs
        jobs={operations.failed_jobs ?? []}
        total={operations.failed}
      />
      <section className="section-block">
        <div className="section-heading">
          <div>
            <h2>Blocked lesson generation</h2>
            <p>
              Chronological order is preserved. Free providers retry
              automatically; using OpenAI always needs this explicit action.
            </p>
          </div>
        </div>
        {operations.blocked_jobs?.length ? (
          <ol className="history-list">
            {operations.blocked_jobs.map((job) => (
              <li key={job.id}>
                <div>
                  <span className="history-topic">
                    Attempt {job.attempt_count}
                  </span>
                  <h3>{job.id}</h3>
                  <p>{formatIndiaDateTime(job.created_at)}</p>
                </div>
                <button
                  className="button button-secondary"
                  disabled={Boolean(openAIPendingJob)}
                  onClick={() => void runOpenAI(job.id)}
                  type="button"
                >
                  {openAIPendingJob === job.id
                    ? "Running with OpenAI…"
                    : "Use OpenAI once"}
                </button>
              </li>
            ))}
          </ol>
        ) : (
          <div className="empty-state">
            <span className="empty-symbol">0</span>
            <div>
              <h3>No quota-blocked work</h3>
              <p>Groq and Gemini are serving the chronological queue.</p>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}

function SourceHealthPanel({ sources }: { sources: SourceHealth[] }) {
  const counts = sources.reduce(
    (totals, source) => {
      totals[source.poll_state] += 1;
      return totals;
    },
    { healthy: 0, degraded: 0, failed: 0, never: 0 },
  );
  const attention = counts.degraded + counts.failed + counts.never;

  return (
    <section className="section-block source-health-section">
      <details className="source-health-disclosure">
        <summary>
          <div>
            <span className="eyebrow">Radar ingestion</span>
            <h2>Source health</h2>
            <p>
              {counts.healthy} healthy · {counts.degraded} degraded ·{" "}
              {counts.failed} failed · {counts.never} not yet polled
            </p>
          </div>
          <StatusPill tone={attention ? "attention" : "ready"}>
            {attention ? `${attention} need attention` : "All healthy"}
          </StatusPill>
        </summary>
        {sources.length ? (
          <ol className="source-health-list">
            {sources.map((source) => (
              <SourceHealthRow key={source.id} source={source} />
            ))}
          </ol>
        ) : (
          <p className="muted-copy">No enabled Radar sources were found.</p>
        )}
      </details>
    </section>
  );
}

function SourceHealthRow({ source }: { source: SourceHealth }) {
  const tone =
    source.poll_state === "healthy"
      ? "ready"
      : source.poll_state === "failed"
        ? "error"
        : "attention";
  const stateLabel =
    source.poll_state === "never"
      ? "Not yet polled"
      : source.poll_state[0].toUpperCase() + source.poll_state.slice(1);

  return (
    <li>
      <div className="source-health-heading">
        <div>
          <h3>{source.publisher}</h3>
          <p>
            {source.source_role.replaceAll("_", " ")} ·{" "}
            {source.fetch_method.toUpperCase()}
          </p>
        </div>
        <StatusPill tone={tone}>{stateLabel}</StatusPill>
      </div>
      {source.last_error_summary ? (
        <div className="source-health-error" role="note">
          <p>{source.last_error_summary}</p>
          <code>{source.last_error_code}</code>
        </div>
      ) : null}
      <dl className="source-health-metadata">
        <div>
          <dt>Last poll</dt>
          <dd>{sourceTimestamp(source.last_polled_at)}</dd>
        </div>
        <div>
          <dt>Last success</dt>
          <dd>{sourceTimestamp(source.last_success_at)}</dd>
        </div>
        <div>
          <dt>Latest usable items</dt>
          <dd>{source.last_item_count}</dd>
        </div>
        <div>
          <dt>Items seen in 10 days</dt>
          <dd>{source.recent_items}</dd>
        </div>
      </dl>
    </li>
  );
}

function sourceTimestamp(value: string) {
  return value ? formatIndiaDateTime(value) : "Not available";
}

function LessonNowPanel({
  lesson,
  starting,
  error,
  onSend,
}: {
  lesson: ManualLesson | null;
  starting: boolean;
  error: string;
  onSend: () => void;
}) {
  const active = lesson ? ACTIVE_LESSON_STATES.has(lesson.state) : false;
  const quotaBlocked = lesson?.state === "quota_blocked";
  const state = starting ? "starting" : (lesson?.state ?? "ready");
  const stateMessage = error || lesson?.message || lessonStateMessage(state);

  return (
    <OwnerDeliveryPanel
      labelledBy="lesson-now-title"
      title="Send a lesson now"
      description="Generate the next lesson from your current profile and deliver it to Telegram immediately, with Read and Skip feedback."
      statusLabel={lessonStateLabel(state, Boolean(error))}
      statusTone={lessonStateTone(state, Boolean(error))}
      statusMessage={stateMessage}
      error={Boolean(error)}
      actionLabel={lessonActionLabel(state)}
      disabled={starting || active || quotaBlocked}
      onSend={onSend}
    />
  );
}

function RadarNowPanel({
  radar,
  starting,
  error,
  onSend,
}: {
  radar: ManualRadar | null;
  starting: boolean;
  error: string;
  onSend: () => void;
}) {
  const active = radar ? ACTIVE_RADAR_STATES.has(radar.state) : false;
  const state = starting ? "starting" : (radar?.state ?? "ready");
  const stateMessage = error || radar?.message || radarStateMessage(state);

  return (
    <OwnerDeliveryPanel
      labelledBy="radar-now-title"
      title="Send latest Radar now"
      description="Deliver the number of eligible updates saved in your Radar profile, one sourced Telegram message per update. This acceptance test does not consume today's normal Radar target."
      statusLabel={radarStateLabel(state, Boolean(error))}
      statusTone={radarStateTone(state, Boolean(error))}
      statusMessage={stateMessage}
      error={Boolean(error)}
      actionLabel={radarActionLabel(state)}
      disabled={starting || active}
      onSend={onSend}
    />
  );
}

function OwnerDeliveryPanel({
  labelledBy,
  title,
  description,
  statusLabel,
  statusTone,
  statusMessage,
  error,
  actionLabel,
  disabled,
  onSend,
}: {
  labelledBy: string;
  title: string;
  description: string;
  statusLabel: string;
  statusTone: "neutral" | "ready" | "attention" | "error";
  statusMessage: string;
  error: boolean;
  actionLabel: string;
  disabled: boolean;
  onSend: () => void;
}) {
  return (
    <section className="lesson-now-panel" aria-labelledby={labelledBy}>
      <div className="lesson-now-copy">
        <div>
          <h2 id={labelledBy}>{title}</h2>
          <p>{description}</p>
        </div>
        <div
          className="lesson-now-status"
          role={error ? "alert" : "status"}
          aria-live={error ? "assertive" : "polite"}
        >
          <StatusPill tone={statusTone}>{statusLabel}</StatusPill>
          <p>{statusMessage}</p>
        </div>
      </div>
      <button
        className="button button-primary lesson-now-action"
        disabled={disabled}
        onClick={onSend}
        type="button"
      >
        {actionLabel}
      </button>
    </section>
  );
}

type LessonDisplayState = ManualLesson["state"] | "ready" | "starting";

function lessonStateLabel(state: LessonDisplayState, hasError: boolean) {
  if (hasError) return "Needs attention";
  switch (state) {
    case "ready":
      return "Ready";
    case "starting":
      return "Starting";
    case "queued":
      return "Queued";
    case "generating":
      return "Generating";
    case "delivering":
      return "Delivering";
    case "delivered":
      return "Delivered";
    case "quota_blocked":
      return "Quota blocked";
    case "failed":
      return "Failed";
  }
}

function lessonStateTone(
  state: LessonDisplayState,
  hasError: boolean,
): "neutral" | "ready" | "attention" | "error" {
  if (hasError || state === "failed") return "error";
  if (state === "delivered") return "ready";
  if (state === "quota_blocked") return "attention";
  return "neutral";
}

function lessonStateMessage(state: LessonDisplayState) {
  switch (state) {
    case "ready":
      return "Ready to use your current topics, level, depth, and delivery settings.";
    case "starting":
      return "Creating a durable lesson job…";
    default:
      return "Lesson status is updating.";
  }
}

function lessonActionLabel(state: LessonDisplayState) {
  switch (state) {
    case "starting":
      return "Starting…";
    case "queued":
      return "Lesson queued";
    case "generating":
      return "Generating…";
    case "delivering":
      return "Delivering…";
    case "delivered":
      return "Send another lesson";
    case "quota_blocked":
      return "Waiting for provider";
    case "failed":
      return "Try again";
    default:
      return "Send lesson now";
  }
}

type RadarDisplayState = ManualRadar["state"] | "ready" | "starting";

function radarStateLabel(state: RadarDisplayState, hasError: boolean) {
  if (hasError) return "Needs attention";
  switch (state) {
    case "ready":
      return "Ready";
    case "starting":
      return "Starting";
    case "queued":
      return "Queued";
    case "delivering":
      return "Delivering";
    case "delivered":
      return "Delivered";
    case "failed":
      return "Failed";
  }
}

function radarStateTone(
  state: RadarDisplayState,
  hasError: boolean,
): "neutral" | "ready" | "attention" | "error" {
  if (hasError || state === "failed") return "error";
  if (state === "delivered") return "ready";
  return "neutral";
}

function radarStateMessage(state: RadarDisplayState) {
  switch (state) {
    case "ready":
      return "Ready to send your saved Radar target from the best ranked updates currently available.";
    case "starting":
      return "Reserving the best eligible Radar updates…";
    default:
      return "Radar delivery status is updating.";
  }
}

function radarActionLabel(state: RadarDisplayState) {
  switch (state) {
    case "starting":
      return "Starting…";
    case "queued":
      return "Radar batch queued";
    case "delivering":
      return "Delivering…";
    case "delivered":
      return "Send another batch";
    case "failed":
      return "Try again";
    default:
      return "Send Radar batch";
  }
}

function Metric({
  label,
  value,
  tone,
  detailsHref,
}: {
  label: string;
  value: number;
  tone: "neutral" | "ready" | "attention" | "error";
  detailsHref?: string;
}) {
  return (
    <article className="metric">
      <StatusPill tone={tone}>{label}</StatusPill>
      <strong>{value}</strong>
      {detailsHref ? (
        <a className="metric-link" href={detailsHref}>
          Review failures
        </a>
      ) : null}
    </article>
  );
}

function FailedJobs({ jobs, total }: { jobs: FailedJob[]; total: number }) {
  return (
    <section className="section-block" id="failed-jobs">
      <div className="section-heading">
        <div>
          <h2>Failed jobs</h2>
          <p>
            Terminal failures with safe diagnostics for every recorded attempt.
            Provider payloads, lesson text, credentials, and Telegram
            identifiers are never shown here.
          </p>
        </div>
        {total > jobs.length ? (
          <p className="failure-list-limit">
            Showing the latest {jobs.length} of {total}
          </p>
        ) : null}
      </div>
      {jobs.length ? (
        <ol className="failure-list">
          {jobs.map((job) => (
            <FailedJobDetails job={job} key={job.id} />
          ))}
        </ol>
      ) : (
        <div className="empty-state">
          <span className="empty-symbol">0</span>
          <div>
            <h3>No failed jobs</h3>
            <p>
              Terminal queue failures will appear here with their attempt
              history.
            </p>
          </div>
        </div>
      )}
    </section>
  );
}

function FailedJobDetails({ job }: { job: FailedJob }) {
  const latestFailure = job.failures[0];
  const latestSummary =
    latestFailure?.error_summary ||
    job.last_error_summary ||
    "Detailed diagnostics were not recorded for this historical failure.";

  return (
    <li>
      <details className="failure-details">
        <summary>
          <div className="failure-summary-main">
            <div className="failure-title-row">
              <h3>{jobTypeLabel(job.job_type)}</h3>
              <span>
                Attempt {job.attempt_count} of {job.max_attempts}
              </span>
            </div>
            <p>{latestSummary}</p>
            <time dateTime={job.failed_at}>
              Failed {formatIndiaDateTime(job.failed_at)}
            </time>
          </div>
          <span className="failure-disclosure" aria-hidden="true" />
        </summary>
        <div className="failure-body">
          <dl className="failure-metadata">
            <div>
              <dt>Job ID</dt>
              <dd>
                <code>{job.id}</code>
              </dd>
            </div>
            <div>
              <dt>Latest code</dt>
              <dd>
                <code>{job.last_error_code || "unavailable"}</code>
              </dd>
            </div>
            <div>
              <dt>Queued</dt>
              <dd>
                <time dateTime={job.created_at}>
                  {formatIndiaDateTime(job.created_at)}
                </time>
              </dd>
            </div>
          </dl>
          {job.failures.length ? (
            <>
              {job.failure_count > job.failures.length ? (
                <p className="failure-attempt-limit">
                  Showing the latest {job.failures.length} of{" "}
                  {job.failure_count}
                  recorded attempts.
                </p>
              ) : null}
              <ol className="failure-attempts">
                {job.failures.map((failure, index) => (
                  <FailureAttempt
                    failure={failure}
                    key={`${failure.occurred_at}-${failure.attempt_count}-${index}`}
                  />
                ))}
              </ol>
            </>
          ) : (
            <p className="failure-history-unavailable">
              This job failed before attempt-level diagnostics were enabled. Its
              historical log output cannot be reconstructed safely.
            </p>
          )}
        </div>
      </details>
    </li>
  );
}

function FailureAttempt({ failure }: { failure: JobFailureEvent }) {
  return (
    <li>
      <div className="failure-attempt-heading">
        <strong>Attempt {failure.attempt_count}</strong>
        <time dateTime={failure.occurred_at}>
          {formatIndiaDateTime(failure.occurred_at)}
        </time>
      </div>
      <p>{failure.error_summary}</p>
      <div className="failure-attempt-codes">
        <code>{failure.error_code}</code>
        <span>{failureStateLabel(failure.next_state)}</span>
      </div>
    </li>
  );
}

function jobTypeLabel(jobType: string) {
  const labels: Record<string, string> = {
    generate_lesson: "Generate lesson",
    deliver_lesson: "Deliver lesson",
    ingest_source: "Ingest news source",
    rank_radar: "Rank Radar updates",
    deliver_radar: "Deliver Radar update",
    recover: "Recover queue",
  };
  return labels[jobType] ?? "Background job";
}

function failureStateLabel(state: JobFailureEvent["next_state"]) {
  switch (state) {
    case "queued":
      return "Retried later";
    case "blocked_quota":
      return "Blocked by quota";
    case "failed":
      return "Terminal failure";
  }
}
