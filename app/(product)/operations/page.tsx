"use client";

import { useCallback, useEffect, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { ManualLesson, Operations } from "@/lib/api-types";

const ACTIVE_LESSON_STATES = new Set<ManualLesson["state"]>([
  "queued",
  "generating",
  "delivering",
]);

export default function OperationsPage() {
  const [operations, setOperations] = useState<Operations | null>(null);
  const [inviteURL, setInviteURL] = useState("");
  const [message, setMessage] = useState("");
  const [messageIsError, setMessageIsError] = useState(false);
  const [lesson, setLesson] = useState<ManualLesson | null>(null);
  const [lessonStarting, setLessonStarting] = useState(false);
  const [lessonError, setLessonError] = useState("");

  const refresh = useCallback(async () => {
    setOperations(await api<Operations>("/operations"));
  }, []);

  useEffect(() => {
    let active = true;
    api<Operations>("/operations")
      .then((value) => {
        if (active) setOperations(value);
      })
      .catch((reason: Error) => {
        if (active) setMessage(reason.message);
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
    async function pollLesson() {
      try {
        const value = await api<ManualLesson>(
          `/operations/lessons/${encodeURIComponent(currentJobID)}`,
        );
        if (!active) return;
        setLesson(value);
        setLessonError("");
        if (!ACTIVE_LESSON_STATES.has(value.state)) {
          await refresh();
        }
      } catch (error) {
        if (!active) return;
        setLessonError(
          error instanceof Error
            ? error.message
            : "Lesson status could not be refreshed.",
        );
      }
    }

    const firstPoll = window.setTimeout(() => void pollLesson(), 800);
    const pollInterval = window.setInterval(() => void pollLesson(), 2000);
    return () => {
      active = false;
      window.clearTimeout(firstPoll);
      window.clearInterval(pollInterval);
    };
  }, [lesson?.job_id, lesson?.state, refresh]);

  async function sendLessonNow() {
    setLessonStarting(true);
    setLessonError("");
    try {
      const value = await api<ManualLesson>("/operations/lessons", {
        method: "POST",
        body: JSON.stringify({ request_id: crypto.randomUUID() }),
      });
      setLesson(value);
      await refresh();
    } catch (error) {
      setLessonError(
        error instanceof Error ? error.message : "Lesson creation failed.",
      );
    } finally {
      setLessonStarting(false);
    }
  }

  async function createInvite() {
    try {
      const value = await api<{ url: string }>("/invites", {
        method: "POST",
        body: JSON.stringify({ expires_hours: 168 }),
      });
      setInviteURL(value.url);
      setMessageIsError(false);
      setMessage("A single-use seven-day invite is ready.");
    } catch (error) {
      setMessageIsError(true);
      setMessage(
        error instanceof Error ? error.message : "Invite creation failed.",
      );
    }
  }

  async function runOpenAI(jobID: string) {
    if (
      !window.confirm(
        "Run this one blocked lesson with paid OpenAI credits? This is the only paid-provider path.",
      )
    )
      return;
    try {
      await api<void>("/operations/openai", {
        method: "POST",
        body: JSON.stringify({ job_id: jobID }),
      });
      setMessageIsError(false);
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
      await refresh();
    } catch (error) {
      setMessageIsError(true);
      setMessage(error instanceof Error ? error.message : "OpenAI run failed.");
    }
  }

  if (!operations) {
    return (
      <div className="page-stack">
        <span className="loading-line" />
        <p className="muted-copy">Loading operational state…</p>
        {message ? <p className="field-error">{message}</p> : null}
      </div>
    );
  }

  return (
    <div className="page-stack">
      <PageHeader
        title="Operations"
        description="Owner-only lesson testing, queue truth, private invitations, and the explicit paid-provider boundary."
        action={
          <button
            className="button button-secondary"
            onClick={() => void createInvite()}
          >
            Create invite
          </button>
        }
      />
      <LessonNowPanel
        lesson={lesson}
        starting={lessonStarting}
        error={lessonError}
        onSend={() => void sendLessonNow()}
      />
      {message ? (
        <section
          className={`notice ${messageIsError ? "notice-error" : "notice-success"}`}
          role={messageIsError ? "alert" : "status"}
        >
          <div>
            <strong>
              {messageIsError ? "Owner action failed" : "Owner action complete"}
            </strong>
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
            onClick={() => void navigator.clipboard.writeText(inviteURL)}
          >
            Copy link
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
        />
        <Metric label="Users" value={operations.users} tone="neutral" />
        <Metric label="Sources" value={operations.sources} tone="neutral" />
      </section>
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
                  <p>
                    {new Intl.DateTimeFormat("en-IN", {
                      dateStyle: "medium",
                      timeStyle: "short",
                    }).format(new Date(job.created_at))}
                  </p>
                </div>
                <button
                  className="button button-secondary"
                  onClick={() => void runOpenAI(job.id)}
                >
                  Use OpenAI once
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
    <section className="lesson-now-panel" aria-labelledby="lesson-now-title">
      <div className="lesson-now-copy">
        <div>
          <h2 id="lesson-now-title">Send a lesson now</h2>
          <p>
            Generate the next lesson from your current profile and deliver it to
            Telegram immediately, with Read and Skip feedback.
          </p>
        </div>
        <div
          className="lesson-now-status"
          role={error ? "alert" : "status"}
          aria-live={error ? "assertive" : "polite"}
        >
          <StatusPill tone={lessonStateTone(state, Boolean(error))}>
            {lessonStateLabel(state, Boolean(error))}
          </StatusPill>
          <p>{stateMessage}</p>
        </div>
      </div>
      <button
        className="button button-primary lesson-now-action"
        disabled={starting || active || quotaBlocked}
        onClick={onSend}
      >
        {lessonActionLabel(state)}
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

function Metric({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: "neutral" | "ready" | "attention" | "error";
}) {
  return (
    <article className="metric">
      <StatusPill tone={tone}>{label}</StatusPill>
      <strong>{value}</strong>
    </article>
  );
}
