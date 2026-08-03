"use client";

import { useEffect, useMemo, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { Assignment } from "@/lib/api-types";

export default function ProgressPage() {
  const [assignments, setAssignments] = useState<Assignment[] | null>(null);
  const [dueRecall, setDueRecall] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => {
    api<{ assignments: Assignment[]; due_recall: number }>("/progress")
      .then((value) => {
        setAssignments(value.assignments ?? []);
        setDueRecall(value.due_recall ?? 0);
      })
      .catch((reason: Error) => setError(reason.message));
  }, []);
  const read = useMemo(
    () => assignments?.filter((item) => item.state === "read").length ?? 0,
    [assignments],
  );
  const waiting = useMemo(
    () =>
      assignments?.filter(
        (item) => item.state === "delivered" || item.state === "queued",
      ).length ?? 0,
    [assignments],
  );

  async function mark(item: Assignment, action: "read" | "skip") {
    try {
      setError("");
      await api<void>("/interactions", {
        method: "POST",
        body: JSON.stringify({ assignment_id: item.id, action }),
      });
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Progress could not be saved.",
      );
      return;
    }
    setAssignments(
      (current) =>
        current?.map((value) =>
          value.id === item.id
            ? {
                ...value,
                state: action === "read" ? "read" : "skipped",
                read_at:
                  action === "read" ? new Date().toISOString() : value.read_at,
              }
            : value,
        ) ?? [],
    );
  }

  if (!assignments && !error)
    return (
      <div className="page-stack">
        <span className="loading-line" />
        <p className="muted-copy">Loading learning history…</p>
      </div>
    );
  return (
    <div className="page-stack">
      <PageHeader
        title="Progress"
        description="Evidence of what you have read, skipped, and still have available—without points or streak pressure."
      />
      {error ? (
        <section className="notice notice-error" role="alert">
          <div>
            <strong>History unavailable</strong>
            <p>{error}</p>
          </div>
        </section>
      ) : null}
      <section className="progress-intro">
        <div>
          <StatusPill tone={read > 0 ? "ready" : "neutral"}>
            {read > 0 ? "Learning in motion" : "Awaiting first read"}
          </StatusPill>
          <h2>Progress advances only when you say a lesson was read.</h2>
          <p>
            Unread lessons remain available and never block future delivery.
            Skipping is neutral feedback, not a broken streak.
          </p>
        </div>
        <dl className="progress-facts">
          <div>
            <dt>Read lessons</dt>
            <dd>{read}</dd>
          </div>
          <div>
            <dt>Still available</dt>
            <dd>{waiting}</dd>
          </div>
          <div>
            <dt>Due recall</dt>
            <dd>{dueRecall}</dd>
          </div>
        </dl>
      </section>
      <section className="section-block">
        <div className="section-heading">
          <div>
            <h2>Learning history</h2>
            <p>Newest assignments first.</p>
          </div>
        </div>
        {assignments?.length ? (
          <ol className="history-list">
            {assignments.map((item) => (
              <li key={item.id}>
                <div>
                  <span className="history-topic">{item.topic}</span>
                  <h3>{item.title}</h3>
                  <p>
                    {new Intl.DateTimeFormat("en-IN", {
                      dateStyle: "medium",
                      timeStyle: "short",
                      timeZone: "Asia/Kolkata",
                    }).format(new Date(item.assigned_at))}
                  </p>
                </div>
                <div className="history-action">
                  <StatusPill
                    tone={
                      item.state === "read"
                        ? "ready"
                        : item.state === "skipped"
                          ? "neutral"
                          : "attention"
                    }
                  >
                    {item.state}
                  </StatusPill>
                  {item.state === "delivered" || item.state === "queued" ? (
                    <span className="inline-actions">
                      <button
                        className="text-button"
                        onClick={() => void mark(item, "read")}
                      >
                        Read
                      </button>
                      <button
                        className="text-button"
                        onClick={() => void mark(item, "skip")}
                      >
                        Skip
                      </button>
                    </span>
                  ) : null}
                </div>
              </li>
            ))}
          </ol>
        ) : (
          <div className="empty-state empty-state-wide">
            <span className="empty-symbol">—</span>
            <div>
              <h3>No lessons assigned yet</h3>
              <p>
                The first delivery window will create a theme and send the
                complete lesson to Telegram.
              </p>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
