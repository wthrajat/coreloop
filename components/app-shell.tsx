"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type ReactNode } from "react";

import { AppNavigation } from "@/components/app-navigation";
import { useSession } from "@/components/session-provider";
import { ThemeToggle } from "@/components/theme-toggle";
import { api } from "@/lib/api-client";

export function AppShell({ children }: { children: ReactNode }) {
  const { status, session } = useSession();
  const router = useRouter();
  const [actionError, setActionError] = useState("");
  const [logoutPending, setLogoutPending] = useState(false);
  const logoutActive = useRef(false);

  if (status === "loading") {
    return (
      <main className="centered-state" role="status">
        <span aria-hidden="true" className="loading-line" />
        <p>Loading your private workspace…</p>
      </main>
    );
  }
  if (status === "unauthenticated") {
    return (
      <main className="centered-state">
        <div className="brand-mark">
          <span />
        </div>
        <h1>Telegram sign-in required</h1>
        <p>
          Open your private invite link if this is your first visit. Returning
          members can reconnect directly.
        </p>
        <a
          className="button button-primary"
          href="/api/app/auth/start?return=/overview"
        >
          Reconnect with Telegram
        </a>
        <Link className="text-link" href="/">
          Back to Coreloop
        </Link>
      </main>
    );
  }
  if (status === "error" || !session) {
    return (
      <main className="centered-state">
        <h1>Control surface unavailable</h1>
        <p>
          The API or database is not ready. Your queued Telegram delivery data
          is not changed.
        </p>
        <button
          className="button button-secondary"
          onClick={() => window.location.reload()}
          type="button"
        >
          Try again
        </button>
      </main>
    );
  }

  async function logout() {
    if (logoutActive.current) return;
    logoutActive.current = true;
    setLogoutPending(true);
    try {
      setActionError("");
      await api<void>("/auth/logout", { method: "POST" });
      router.push("/");
      router.refresh();
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "Sign out failed.",
      );
      logoutActive.current = false;
      setLogoutPending(false);
    }
  }

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <aside className="navigation-rail">
        <Link className="brand-lockup" href="/overview">
          <span aria-hidden="true" className="brand-mark">
            <span />
          </span>
          <span>
            <strong>Coreloop</strong>
            <small>Private learning system</small>
          </span>
        </Link>
        <AppNavigation owner={session.owner} />
        <ThemeToggle />
        <div className="rail-status">
          <span className="status-dot status-dot-ready" aria-hidden="true" />
          <span>
            <strong>{session.user.display_name || "Telegram member"}</strong>
            <small>Private profile connected</small>
          </span>
        </div>
        <button
          className="rail-action"
          disabled={logoutPending}
          onClick={() => void logout()}
          type="button"
        >
          {logoutPending ? "Signing out…" : "Sign out"}
        </button>
      </aside>
      <div className="mobile-header">
        <span aria-hidden="true" className="brand-mark brand-mark-small">
          <span />
        </span>
        <strong>Coreloop</strong>
        <ThemeToggle compact />
        <button
          className="mobile-signout"
          disabled={logoutPending}
          onClick={() => void logout()}
          type="button"
        >
          {logoutPending ? "Signing out…" : "Sign out"}
        </button>
      </div>
      <main className="content-canvas" id="main-content" tabIndex={-1}>
        {actionError ? (
          <p className="field-error shell-error" role="alert">
            {actionError}
          </p>
        ) : null}
        {children}
      </main>
      <div className="mobile-navigation">
        <AppNavigation owner={session.owner} />
      </div>
    </div>
  );
}
