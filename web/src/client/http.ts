import type { ErrorResponse } from './types'

// Every API route lives under this prefix (docs/audit.md HTTP-2 —
// versioned API); the unversioned root paths still work but only as
// redirects kept for the transition. absoluteApiUrl below is the form a
// caller uses when it builds a URL for something other than fetch (geo.ts's
// tile template).
const API_BASE = '/api/v1'

// Defaults to "" (paths resolve relative to the page's own origin), which
// is what the admin UI needs since it's served by the same Darow instance
// it talks to. Any other caller — a different web app, a different
// framework — points the client at a remote Darow instance by calling this
// once at startup.
let baseUrl = ''

export function configureApiClient(options: { baseUrl?: string }): void {
    baseUrl = options.baseUrl?.replace(/\/+$/, '') ?? ''
}

export function apiUrl(path: string): string {
    return `${baseUrl}${API_BASE}${path}`
}

// The absolute form of apiUrl, for a URL handed to something that refuses a
// relative one — the map's tile template. The origin is prefixed by hand
// rather than through URL parsing, which would percent-encode the
// {z}/{x}/{y} placeholders and leave the tile source nothing to substitute. A
// client pointed at a remote instance already has an absolute baseUrl.
export function absoluteApiUrl(path: string): string {
    const url = apiUrl(path)
    return /^https?:\/\//.test(url) ? url : `${window.location.origin}${url}`
}

// rootUrl builds a URL outside the /api/v1 prefix — only /healthz and
// /readyz live there (see ops.ts).
function rootUrl(path: string): string {
    return `${baseUrl}${path}`
}

export class ApiError extends Error {
    code: number
    requestId: string

    constructor(response: ErrorResponse) {
        super(response.message)
        this.code = response.code
        this.requestId = response.requestId
    }
}

// statusPhrase gives an ApiError's status class a short French framing —
// the server itself is English-only (spec/back/spec.md has no i18n
// concept), but every user-visible string here must be French (§0.17), and
// a raw "Internal Server Error"/"Not Found" reads as untranslated in an
// otherwise all-French interface.
function statusPhrase(code: number): string {
    if (code === 401) return 'Connexion requise'
    if (code === 403) return 'Action non autorisée'
    if (code === 404) return 'Introuvable'
    if (code === 409) return 'Conflit'
    if (code === 413) return 'Fichier ou requête trop volumineux'
    if (code === 499) return 'Requête annulée'
    if (code === 503) return 'Service indisponible'
    if (code === 504) return 'Délai dépassé'
    if (code >= 500) return 'Erreur interne du serveur'
    if (code >= 400) return 'Requête invalide'
    return ''
}

// errorMessage turns any caught value into a user-facing string: a French
// status phrase plus the server's own detail for an ApiError (the detail is
// kept — even in English, a DuckDB/Go message is usually the most precise
// thing available, per spec/back/spec.md §6.4), or fallback plus the
// underlying error's message for anything else (e.g. a network failure —
// the request never reached the server at all, so there is no ApiError to
// unwrap, but the browser's own error text, "Failed to fetch" and the like,
// is still more diagnosable than a bare generic string). The request
// identifier, when the server returned one, is appended so the text stays
// selectable and quotable when reporting a failure (§8.4).
export function errorMessage(e: unknown, fallback: string): string {
    if (e instanceof ApiError) {
        const phrase = statusPhrase(e.code)
        const base = phrase && e.message ? `${phrase} : ${e.message}` : phrase || e.message
        return base
    }
    if (e instanceof Error) return `${fallback} : ${e.message}`
    return fallback
}

async function handle<T>(res: Response): Promise<T> {
    if (!res.ok) {
        const body = (await res.json().catch(() => null)) as ErrorResponse | null
        throw new ApiError(
            body ?? { error: 'request_error', message: '', code: res.status, requestId: '' }
        )
    }
    if (res.status === 204) {
        return undefined as T
    }
    // /healthz and /readyz answer 200 with no body at all — text() first
    // avoids res.json() throwing on the empty string.
    const text = await res.text()
    return (text ? JSON.parse(text) : undefined) as T
}

// signal lets a caller drop a request it no longer wants — a map load the
// next pan has already superseded — and, with AbortSignal.timeout, stops a
// request that never answers from leaving a screen busy for ever.
export function get<T>(path: string, signal?: AbortSignal): Promise<T> {
    return fetch(apiUrl(path), { signal }).then((res) => handle<T>(res))
}

// getRoot is get's counterpart for the handful of routes outside
// /api/v1 (ops.ts's healthz/readyz).
export function getRoot<T>(path: string): Promise<T> {
    return fetch(rootUrl(path)).then((res) => handle<T>(res))
}

export function post<T>(path: string, body?: unknown): Promise<T> {
    return fetch(apiUrl(path), {
        method: 'POST',
        headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body)
    }).then((res) => handle<T>(res))
}

export function patch<T>(path: string, body: unknown): Promise<T> {
    return fetch(apiUrl(path), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    }).then((res) => handle<T>(res))
}

export function del<T>(path: string): Promise<T> {
    return fetch(apiUrl(path), { method: 'DELETE' }).then((res) => handle<T>(res))
}

// No Content-Type header here — the browser sets the multipart boundary
// itself from the FormData body.
export function postForm<T>(path: string, form: FormData): Promise<T> {
    return fetch(apiUrl(path), { method: 'POST', body: form }).then((res) => handle<T>(res))
}

// getBlob is get's counterpart for a non-JSON response body (tables.ts's
// exportTable) — a failure still becomes the same ApiError every other
// call throws, instead of a plain <a href> download the client never sees.
// The file name comes back with the body: the server is what decides it,
// and for an export it carries the CRS the file is written in (back LIM-16),
// which rebuilding it here would silently drop.
export async function getBlob(path: string): Promise<{ blob: Blob; filename: string | null }> {
    const res = await fetch(apiUrl(path))
    if (!res.ok) {
        const body = (await res.json().catch(() => null)) as ErrorResponse | null
        throw new ApiError(
            body ?? { error: 'request_error', message: '', code: res.status, requestId: '' }
        )
    }
    return { blob: await res.blob(), filename: filenameOf(res.headers.get('Content-Disposition')) }
}

// filenameOf reads the name out of a Content-Disposition header, null when
// the header is absent or names nothing.
function filenameOf(header: string | null): string | null {
    if (!header) return null
    const match = /filename="([^"]+)"/.exec(header)
    return match ? match[1] : null
}
