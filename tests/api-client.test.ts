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

test("api sends the CSRF cookie only on mutating requests", async (context) => {
  const originalFetch = globalThis.fetch;
  const originalDocument = Object.getOwnPropertyDescriptor(
    globalThis,
    "document",
  );
  context.after(() => {
    globalThis.fetch = originalFetch;
    if (originalDocument) {
      Object.defineProperty(globalThis, "document", originalDocument);
    } else {
      Reflect.deleteProperty(globalThis, "document");
    }
  });
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: { cookie: "coreloop_csrf=csrf-token%2Bvalue" },
  });

  const capturedHeaders: Headers[] = [];
  globalThis.fetch = async (_input, init) => {
    capturedHeaders.push(new Headers(init?.headers));
    return new Response(null, { status: 204 });
  };

  await api("/profile");
  await api("/profile", { method: "PUT", body: JSON.stringify({}) });

  assert.equal(capturedHeaders[0].has("X-CSRF-Token"), false);
  assert.equal(capturedHeaders[1].get("X-CSRF-Token"), "csrf-token+value");
});
