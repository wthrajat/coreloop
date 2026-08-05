"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { api } from "@/lib/api-client";
import type { Assignment } from "@/lib/api-types";
import { formatIndiaDateTime } from "@/lib/date-time";

async function fetchProgress() {
  return api<{ assignments: Assignment[] }>("/progress");
}

export default function ProgressPage() {
  const [assignments, setAssignments] = useState<Assignment[] | null>(null);
  const [loadState, setLoadState] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [loadError, setLoadError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingActions, setPendingActions] = useState<
    Record<string, "read" | "skip">
  >({});
  const pendingAssignmentIDs = useRef(new Set<string>());

  const loadProgress = useCallback(async () => {
    try {
      const value = await fetchProgress();
      setAssignments(value.assignments ?? []);
      setLoadState("ready");
    } catch (reason) {
      setLoadError(
        reason instanceof Error
          ? reason.message
          : "Learning history could not be loaded.",
      );
      setLoadState("error");
    }
  }, []);

  useEffect(() => {
    let active = true;
    fetchProgress()
      .then((value) => {
        if (!active) return;
        setAssignments(value.assignments ?? []);
        setLoadState("ready");
      })
      .catch((reason: unknown) => {
        if (!active) return;
        setLoadError(
          reason instanceof Error
            ? reason.message
            : "Learning history could not be loaded.",
        );
        setLoadState("error");
      });
    return () => {
      active = false;
    };
  }, []);

  function retryProgress() {
    setLoadState("loading");
    setLoadError("");
    void loadProgress();
  }
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
    if (pendingAssignmentIDs.current.has(item.id)) return;
    pendingAssignmentIDs.current.add(item.id);
    setPendingActions((current) => ({ ...current, [item.id]: action }));
    try {
      setActionError("");
      await api<void>("/interactions", {
        method: "POST",
        body: JSON.stringify({ assignment_id: item.id, action }),
      });
      setAssignments(
        (current) =>
          current?.map((value) =>
            value.id === item.id
              ? {
                  ...value,
                  state: action === "read" ? "read" : "skipped",
                  read_at:
                    action === "read"
                      ? new Date().toISOString()
                      : value.read_at,
                }
              : value,
          ) ?? [],
      );
    } catch (reason) {
      setActionError(
        reason instanceof Error
          ? reason.message
          : "Progress could not be saved.",
      );
    } finally {
      pendingAssignmentIDs.current.delete(item.id);
      setPendingActions((current) => {
        const next = { ...current };
        delete next[item.id];
        return next;
      });
    }
  }

  if (loadState === "loading")
    return (
      <div className="page-stack" role="status">
        <span aria-hidden="true" className="loading-line" />
        <p className="muted-copy">Loading learning history…</p>
      </div>
    );
  if (loadState === "error" || !assignments) {
    return (
      <div className="page-stack">
        <PageHeader
          title="Progress"
          description="Your saved learning history has not been changed."
        />
        <section className="notice notice-error" role="alert">
          <div>
            <strong>History unavailable</strong>
            <p>{loadError || "Learning history could not be loaded."}</p>
          </div>
          <button
            className="button button-secondary"
            onClick={retryProgress}
            type="button"
          >
            Try again
          </button>
        </section>
      </div>
    );
  }
  return (
    <div className="page-stack">
      <PageHeader
        title="Progress"
        description="Evidence of what you have read, skipped, and still have available—without points or streak pressure."
      />
      {actionError ? (
        <section className="notice notice-error" role="alert">
          <div>
            <strong>Progress could not be saved</strong>
            <p>{actionError}</p>
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
                  <p>{formatIndiaDateTime(item.assigned_at)}</p>
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
                        disabled={Boolean(pendingActions[item.id])}
                        onClick={() => void mark(item, "read")}
                        type="button"
                      >
                        {pendingActions[item.id] === "read"
                          ? "Saving…"
                          : "Read"}
                      </button>
                      <button
                        className="text-button"
                        disabled={Boolean(pendingActions[item.id])}
                        onClick={() => void mark(item, "skip")}
                        type="button"
                      >
                        {pendingActions[item.id] === "skip"
                          ? "Saving…"
                          : "Skip"}
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
