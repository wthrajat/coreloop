"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { Overview } from "@/lib/api-types";

export default function OverviewPage() {
  const [overview, setOverview] = useState<Overview | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api<Overview>("/overview")
      .then(setOverview)
      .catch((reason: Error) => setError(reason.message));
  }, []);

  if (!overview && !error)
    return <PageLoading label="Loading your learning rhythm…" />;
  if (error) return <PageError message={error} />;
  if (!overview) return null;

  const nextDelivery = overview.next_delivery_at
    ? new Intl.DateTimeFormat("en-IN", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: "Asia/Kolkata",
      }).format(new Date(overview.next_delivery_at))
    : "Waiting for schedule";
  const quotaBlocked = overview.queue_state === "quota_exhausted";

  return (
    <div className="page-stack">
      <PageHeader
        title="Overview"
        description="Your learning rhythm, delivery state, and current theme in one place."
        action={
          <Link className="button button-secondary" href="/settings">
            Adjust learning
          </Link>
        }
      />
      {quotaBlocked ? (
        <section className="notice notice-error" role="alert">
          <div>
            <strong>Free AI quota is exhausted</strong>
            <p>
              {overview.quota_blocked_count} lesson job
              {overview.quota_blocked_count === 1 ? " is" : "s are"} safely
              queued. Nothing was sent to paid OpenAI automatically.
            </p>
          </div>
        </section>
      ) : null}
      <section className="setup-panel" aria-labelledby="theme-title">
        <div className="setup-copy">
          <StatusPill tone={overview.active_theme ? "ready" : "attention"}>
            {overview.active_theme ? "Theme active" : "Planner ready"}
          </StatusPill>
          <h2 id="theme-title">
            {overview.active_theme ||
              "Your first coherent theme starts at the next delivery."}
          </h2>
          <p>
            {overview.active_topic
              ? `Current topic: ${overview.active_topic}. Lessons continue from foundations through production decisions before the planner moves on.`
              : "Your selected topics, level, duration, and recent coverage determine the first theme. The choice is saved before generation so topics do not repeat needlessly."}
          </p>
          <div className="button-row">
            <Link className="button button-primary" href="/settings">
              Review configuration
            </Link>
            <Link className="text-link" href="/progress">
              See learning history
            </Link>
          </div>
        </div>
        <div className="schedule-preview">
          <div className="section-heading compact-heading">
            <h3>Next curriculum delivery</h3>
            <span>Asia/Kolkata</span>
          </div>
          <p className="next-delivery">{nextDelivery}</p>
          <dl className="compact-stats">
            <div>
              <dt>Waiting to be read</dt>
              <dd>{overview.unread_lessons}</dd>
            </div>
            <div>
              <dt>Read</dt>
              <dd>{overview.read_lessons}</dd>
            </div>
          </dl>
          <p className="schedule-caption">
            Radar signals are ranked separately and may arrive outside this
            window.
          </p>
        </div>
      </section>
      <div className="overview-columns">
        <section className="section-block">
          <div className="section-heading">
            <div>
              <h2>Learning state</h2>
              <p>Delivery does not count as progress until you press Read.</p>
            </div>
          </div>
          <dl className="status-list">
            <div>
              <dt>Active theme</dt>
              <dd>{overview.active_theme || "Starts automatically"}</dd>
            </div>
            <div>
              <dt>Unread backlog</dt>
              <dd>{overview.unread_lessons} available</dd>
            </div>
            <div>
              <dt>Completed lessons</dt>
              <dd>{overview.read_lessons}</dd>
            </div>
          </dl>
        </section>
        <aside className="section-block system-block">
          <div className="section-heading">
            <div>
              <h2>Delivery system</h2>
              <p>Connection and queue state only.</p>
            </div>
          </div>
          <dl className="status-list">
            <div>
              <dt>Telegram</dt>
              <dd>
                <StatusPill
                  tone={overview.telegram_connected ? "ready" : "error"}
                >
                  {overview.telegram_connected ? "Connected" : "Disconnected"}
                </StatusPill>
              </dd>
            </div>
            <div>
              <dt>Curriculum queue</dt>
              <dd>
                <StatusPill tone={quotaBlocked ? "error" : "ready"}>
                  {quotaBlocked ? "Waiting for quota" : "Healthy"}
                </StatusPill>
              </dd>
            </div>
            <div>
              <dt>Automatic providers</dt>
              <dd>Groq → Gemini</dd>
            </div>
            <div>
              <dt>Paid OpenAI</dt>
              <dd>Owner action only</dd>
            </div>
          </dl>
        </aside>
      </div>
    </div>
  );
}

function PageLoading({ label }: { label: string }) {
  return (
    <div className="page-stack">
      <span className="loading-line" />
      <p className="muted-copy">{label}</p>
    </div>
  );
}
function PageError({ message }: { message: string }) {
  return (
    <div className="page-stack">
      <section className="notice notice-error" role="alert">
        <div>
          <strong>Overview unavailable</strong>
          <p>{message}</p>
        </div>
        <button
          className="button button-secondary"
          onClick={() => window.location.reload()}
        >
          Try again
        </button>
      </section>
    </div>
  );
}
