"use client";

import { useEffect, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { Operations } from "@/lib/api-types";

export default function OperationsPage() {
  const [operations, setOperations] = useState<Operations | null>(null);
  const [inviteURL, setInviteURL] = useState("");
  const [message, setMessage] = useState("");
  const [messageIsError, setMessageIsError] = useState(false);

  async function refresh() {
    setOperations(await api<Operations>("/operations"));
  }

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
        description="Owner-only queue truth, private invitations, and the explicit paid-provider boundary."
        action={
          <button
            className="button button-primary"
            onClick={() => void createInvite()}
          >
            Create invite
          </button>
        }
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
