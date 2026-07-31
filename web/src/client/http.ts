import type { ErrorResponse } from "./types";

// Every API route lives under this prefix (docs/audit.md HTTP-2 —
// versioned API); the unversioned root paths still work but only as
// redirects kept for the transition. apiUrl is also used directly by
// callers building a URL for <a href>/tile templates rather than going
// through fetch (tables.ts's exportUrl, geo.ts's tileUrl).
const API_BASE = "/api/v1";

// Defaults to "" (paths resolve relative to the page's own origin), which
// is what the admin UI needs since it's served by the same Darow instance
// it talks to. Any other caller — a different web app, a different
// framework — points the client at a remote Darow instance by calling this
// once at startup.
let baseUrl = "";

export function configureApiClient(options: { baseUrl?: string }): void {
  baseUrl = options.baseUrl?.replace(/\/+$/, "") ?? "";
}

export function apiUrl(path: string): string {
  return `${baseUrl}${API_BASE}${path}`;
}

// rootUrl builds a URL outside the /api/v1 prefix — only /healthz and
// /readyz live there (see ops.ts).
function rootUrl(path: string): string {
  return `${baseUrl}${path}`;
}

export class ApiError extends Error {
  code: number;

  constructor(response: ErrorResponse) {
    super(response.error);
    this.code = response.code;
  }
}

// errorMessage turns any caught value into a user-facing string: the
// server's own message for an ApiError, or fallback plus the underlying
// error's message for anything else (e.g. a network failure — the request
// never reached the server at all, so there is no ApiError to unwrap, but
// the browser's own error text, "Failed to fetch" and the like, is still
// more diagnosable than a bare generic string).
export function errorMessage(e: unknown, fallback: string): string {
  if (e instanceof ApiError) return e.message;
  if (e instanceof Error) return `${fallback} : ${e.message}`;
  return fallback;
}

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ErrorResponse | null;
    throw new ApiError(body ?? { error: res.statusText, message: res.statusText, code: res.status });
  }
  if (res.status === 204) {
    return undefined as T;
  }
  // /healthz and /readyz answer 200 with no body at all — text() first
  // avoids res.json() throwing on the empty string.
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

export function get<T>(path: string): Promise<T> {
  return fetch(apiUrl(path)).then((res) => handle<T>(res));
}

// getRoot is get's counterpart for the handful of routes outside
// /api/v1 (ops.ts's healthz/readyz).
export function getRoot<T>(path: string): Promise<T> {
  return fetch(rootUrl(path)).then((res) => handle<T>(res));
}

export function post<T>(path: string, body?: unknown): Promise<T> {
  return fetch(apiUrl(path), {
    method: "POST",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  }).then((res) => handle<T>(res));
}

export function patch<T>(path: string, body: unknown): Promise<T> {
  return fetch(apiUrl(path), {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  }).then((res) => handle<T>(res));
}

export function del<T>(path: string): Promise<T> {
  return fetch(apiUrl(path), { method: "DELETE" }).then((res) => handle<T>(res));
}

// No Content-Type header here — the browser sets the multipart boundary
// itself from the FormData body.
export function postForm<T>(path: string, form: FormData): Promise<T> {
  return fetch(apiUrl(path), { method: "POST", body: form }).then((res) => handle<T>(res));
}
