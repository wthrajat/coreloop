"use client";

import { useState } from "react";

import { PageHeader } from "@/components/page-header";
import { api } from "@/lib/api-client";

export default function ProfilePage() {
  const [message, setMessage] = useState("");
  async function exportData() {
    try {
      const value = await api<Record<string, unknown>>("/export");
      const blob = new Blob([JSON.stringify(value, null, 2)], {
        type: "application/json",
      });
      const href = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = href;
      link.download = "coreloop-export.json";
      link.click();
      URL.revokeObjectURL(href);
      setMessage("Export downloaded.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Export failed.");
    }
  }
  async function deleteAccount() {
    if (
      !window.confirm(
        "Permanently delete this profile, preferences, progress, delivery history, and Telegram connection? This cannot be undone.",
      )
    )
      return;
    try {
      await api<void>("/account", { method: "DELETE" });
      window.location.assign("/");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Deletion failed.");
    }
  }
  return (
    <div className="page-stack">
      <PageHeader
        title="Profile and privacy"
        description="Export the information owned by your profile or permanently remove it from this instance."
      />
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
            onClick={() => void exportData()}
          >
            Download JSON export
          </button>
          {message ? <p role="status">{message}</p> : null}
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
            onClick={() => void deleteAccount()}
          >
            Delete my profile
          </button>
        </div>
      </section>
    </div>
  );
}
