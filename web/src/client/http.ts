import type { ErrorResponse } from "./types";

export class ApiError extends Error {
  code: number;

  constructor(response: ErrorResponse) {
    super(response.error);
    this.code = response.code;
  }
}

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ErrorResponse | null;
    throw new ApiError(body ?? { error: res.statusText, message: res.statusText, code: res.status });
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export function get<T>(path: string): Promise<T> {
  return fetch(path).then((res) => handle<T>(res));
}

export function post<T>(path: string, body?: unknown): Promise<T> {
  return fetch(path, {
    method: "POST",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  }).then((res) => handle<T>(res));
}

export function patch<T>(path: string, body: unknown): Promise<T> {
  return fetch(path, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  }).then((res) => handle<T>(res));
}

export function del<T>(path: string): Promise<T> {
  return fetch(path, { method: "DELETE" }).then((res) => handle<T>(res));
}

// No Content-Type header here — the browser sets the multipart boundary
// itself from the FormData body.
export function postForm<T>(path: string, form: FormData): Promise<T> {
  return fetch(path, { method: "POST", body: form }).then((res) => handle<T>(res));
}
