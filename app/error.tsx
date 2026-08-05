"use client";

import { useEffect } from "react";

export default function ApplicationError({
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
    <main className="centered-state">
      <span className="eyebrow">Coreloop</span>
      <h1>This page could not be opened.</h1>
      <p>
        Your saved profile and queued Telegram deliveries are unchanged. Try the
        page again, or return after the service recovers.
      </p>
      <button className="button button-primary" onClick={reset} type="button">
        Try again
      </button>
    </main>
  );
}
