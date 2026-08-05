"use client";

import { useRef, useState } from "react";

import { PageHeader } from "@/components/page-header";
import { api } from "@/lib/api-client";

export default function ProfilePage() {
  const [message, setMessage] = useState("");
  const [messageIsError, setMessageIsError] = useState(false);
  const [pendingAction, setPendingAction] = useState<"export" | "delete" | "">(
    "",
  );
  const actionActive = useRef(false);
  async function exportData() {
    if (actionActive.current) return;
    actionActive.current = true;
    setPendingAction("export");
    setMessage("");
    try {
      const value = await api<Record<string, unknown>>("/export");
      const blob = new Blob([JSON.stringify(value, null, 2)], {
        type: "application/json",
      });
      const href = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = href;
      link.download = "coreloop-export.json";
      document.body.append(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(href), 0);
      setMessageIsError(false);
      setMessage("Export downloaded.");
    } catch (error) {
      setMessageIsError(true);
      setMessage(error instanceof Error ? error.message : "Export failed.");
    } finally {
      actionActive.current = false;
      setPendingAction("");
    }
  }
  async function deleteAccount() {
    if (actionActive.current) return;
    if (
      !window.confirm(
        "Permanently delete this profile, preferences, progress, delivery history, and Telegram connection? This cannot be undone.",
      )
    )
      return;
    actionActive.current = true;
    setPendingAction("delete");
    setMessage("");
    try {
      await api<void>("/account", { method: "DELETE" });
      window.location.assign("/");
    } catch (error) {
      setMessageIsError(true);
      setMessage(error instanceof Error ? error.message : "Deletion failed.");
      actionActive.current = false;
      setPendingAction("");
    }
  }
  return (
    <div className="page-stack">
      <PageHeader
        title="Profile and privacy"
        description="Export the information owned by your profile or permanently remove it from this instance."
      />
      {message ? (
        <section
          className={`notice ${messageIsError ? "notice-error" : "notice-success"}`}
          role={messageIsError ? "alert" : "status"}
        >
          <div>
            <strong>
              {messageIsError ? "Action failed" : "Action complete"}
            </strong>
            <p>{message}</p>
          </div>
        </section>
      ) : null}
      <section className="settings-section">
        <div className="settings-heading">
          <h2>Portable data</h2>
          <p>
            The export contains profile choices, selected topics, preferences,
            and assignment history. It never contains provider keys or another
            user&apos;s data.
          </p>
        </div>
        <div className="action-panel">
          <button
            className="button button-secondary"
            disabled={Boolean(pendingAction)}
            onClick={() => void exportData()}
            type="button"
          >
            {pendingAction === "export"
              ? "Preparing export…"
              : "Download JSON export"}
          </button>
        </div>
      </section>
      <section className="settings-section danger-section">
        <div className="settings-heading">
          <h2>Delete profile</h2>
          <p>
            This permanently removes your private profile and cascades its
            schedules, progress, jobs, and delivery connection. Shared anonymous
            lessons remain available to the cache.
          </p>
        </div>
        <div className="action-panel">
          <button
            className="button button-danger"
            disabled={Boolean(pendingAction)}
            onClick={() => void deleteAccount()}
            type="button"
          >
            {pendingAction === "delete"
              ? "Deleting profile…"
              : "Delete my profile"}
          </button>
        </div>
      </section>
    </div>
  );
}
