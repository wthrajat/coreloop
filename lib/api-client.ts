export class APIError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

export type APIRequestInit = RequestInit & {
  timeoutMs?: number;
};

const defaultRequestTimeoutMs = 20_000;

function cookie(name: string): string {
  if (typeof document === "undefined") return "";
  const value = document.cookie
    .split("; ")
    .find((item) => item.startsWith(`${name}=`));
  return value ? decodeURIComponent(value.slice(name.length + 1)) : "";
}

export async function api<T>(
  path: string,
  options: APIRequestInit = {},
): Promise<T> {
  const { timeoutMs = defaultRequestTimeoutMs, ...requestOptions } = options;
  const method = requestOptions.method?.toUpperCase() ?? "GET";
  const headers = new Headers(requestOptions.headers);
  if (requestOptions.body) headers.set("Content-Type", "application/json");
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    headers.set("X-CSRF-Token", cookie("coreloop_csrf"));
  }

  const requestController = new AbortController();
  const sourceSignal = requestOptions.signal;
  const abortFromSource = () => requestController.abort(sourceSignal?.reason);
  if (sourceSignal?.aborted) abortFromSource();
  else sourceSignal?.addEventListener("abort", abortFromSource, { once: true });

  let timedOut = false;
  const timeoutID = globalThis.setTimeout(() => {
    timedOut = true;
    requestController.abort();
  }, timeoutMs);

  let response: Response;
  try {
    response = await fetch(`/api/app${path}`, {
      ...requestOptions,
      headers,
      credentials: "same-origin",
      cache: "no-store",
      signal: requestController.signal,
    });
  } catch (error) {
    if (timedOut) {
      throw new APIError(
        408,
        "request_timeout",
        "The request took too long. Please try again.",
      );
    }
    throw error;
  } finally {
    globalThis.clearTimeout(timeoutID);
    sourceSignal?.removeEventListener("abort", abortFromSource);
  }
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as {
      error?: { code?: string; message?: string };
    } | null;
    throw new APIError(
      response.status,
      payload?.error?.code ?? "request_failed",
      payload?.error?.message ?? "The request could not be completed.",
    );
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}
