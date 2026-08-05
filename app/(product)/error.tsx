"use client";

import { useEffect } from "react";

import { PageHeader } from "@/components/page-header";

export default function ProductError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="page-stack">
      <PageHeader
        title="Control surface unavailable"
        description="Your saved profile and queued Telegram deliveries are unchanged."
      />
      <section className="notice notice-error" role="alert">
        <div>
          <strong>This page stopped unexpectedly</strong>
          <p>Try loading it again. No learning or delivery data was removed.</p>
        </div>
        <button
          className="button button-secondary"
          onClick={reset}
          type="button"
        >
          Try again
        </button>
      </section>
    </div>
  );
}
