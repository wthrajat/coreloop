import assert from "node:assert/strict";
import test from "node:test";

import { api, APIError } from "../lib/api-client.ts";

test("api preserves structured server errors", async (context) => {
  const originalFetch = globalThis.fetch;
  context.after(() => {
    globalThis.fetch = originalFetch;
  });
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({
        error: { code: "not_ready", message: "Service is warming up." },
      }),
      { status: 503, headers: { "Content-Type": "application/json" } },
    );

  await assert.rejects(
    api("/test"),
    (error: unknown) =>
      error instanceof APIError &&
      error.status === 503 &&
      error.code === "not_ready" &&
      error.message === "Service is warming up.",
  );
});

test("api turns a request timeout into an actionable error", async (context) => {
  const originalFetch = globalThis.fetch;
  context.after(() => {
    globalThis.fetch = originalFetch;
  });
  globalThis.fetch = (_input, init) =>
    new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener(
        "abort",
        () => reject(new DOMException("Aborted", "AbortError")),
        { once: true },
      );
    });

  await assert.rejects(
    api("/slow", { timeoutMs: 5 }),
    (error: unknown) =>
      error instanceof APIError &&
      error.status === 408 &&
      error.code === "request_timeout",
  );
});
