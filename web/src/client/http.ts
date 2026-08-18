import type { ErrorResponse } from './types'

// Every API route lives under this prefix; the unversioned root paths survive
// only as redirects. absoluteApiUrl below is the form a caller uses to build a
// URL for something other than fetch (geo.ts's tile template).
const API_BASE = '/api/v1'

// Defaults to "" — paths resolve against the page's own origin, which is what
// the admin UI needs, being served by the Darow instance it talks to. Any
// other caller points at a remote instance by calling this once at startup.
let baseUrl = ''

export function configureApiClient(options: { baseUrl?: string }): void {
    baseUrl = options.baseUrl?.replace(/\/+$/, '') ?? ''
}

// The bearer token every request attaches, and its single setter
// (composables/useAuth.ts). http.ts never reads the auth composable itself:
// it only ever sees a string or null here.
let authToken: string | null = null

export function setAuthToken(token: string | null): void {
    authToken = token
}

// A 401 means the held token is no longer good. http.ts cannot decide what to
// do about it, so it only notifies whoever registered a handler — importing
// the auth composable would pull Vue reactivity into the transport layer.
let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(handler: (() => void) | null): void {
    onUnauthorized = handler
}

function authHeaders(): HeadersInit {
    return authToken ? { Authorization: `Bearer ${authToken}` } : {}
}

export function apiUrl(path: string): string {
    return `${baseUrl}${API_BASE}${path}`
}

// The absolute form of apiUrl, for whatever refuses a relative URL — the map's
// tile template. The origin is prefixed by hand: URL parsing would
// percent-encode the {z}/{x}/{y} placeholders the tile source substitutes.
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

// statusPhrase gives an ApiError's status class a short French framing: the
// server is English-only, but every user-visible string here must be French
// (§0.17), and a raw "Not Found" reads as untranslated in it.
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
// status phrase plus the server's own detail for an ApiError, the fallback
// plus the underlying message for anything else (a network failure has no
// ApiError to unwrap). The request id, when there is one, is appended (§8.4).
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
    if (res.status === 401) {
        onUnauthorized?.()
    }
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
    return fetch(apiUrl(path), { headers: authHeaders(), signal }).then((res) => handle<T>(res))
}

// getRoot is get's counterpart for the handful of routes outside
// /api/v1 (ops.ts's healthz/readyz).
export function getRoot<T>(path: string): Promise<T> {
    return fetch(rootUrl(path)).then((res) => handle<T>(res))
}

export function post<T>(path: string, body?: unknown): Promise<T> {
    return fetch(apiUrl(path), {
        method: 'POST',
        headers: {
            ...authHeaders(),
            ...(body === undefined ? {} : { 'Content-Type': 'application/json' })
        },
        body: body === undefined ? undefined : JSON.stringify(body)
    }).then((res) => handle<T>(res))
}

export function patch<T>(path: string, body: unknown): Promise<T> {
    return fetch(apiUrl(path), {
        method: 'PATCH',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    }).then((res) => handle<T>(res))
}

export function del<T>(path: string): Promise<T> {
    return fetch(apiUrl(path), { method: 'DELETE', headers: authHeaders() }).then((res) =>
        handle<T>(res)
    )
}

// No Content-Type header here — the browser sets the multipart boundary
// itself from the FormData body.
export function postForm<T>(path: string, form: FormData): Promise<T> {
    return fetch(apiUrl(path), { method: 'POST', headers: authHeaders(), body: form }).then((res) =>
        handle<T>(res)
    )
}

// get's counterpart for a non-JSON body (tables.ts's exportTable): a failure
// still becomes the same ApiError every other call throws. The file name comes
// back with the body — it carries the export's CRS (back LIM-16).
export async function getBlob(path: string): Promise<{ blob: Blob; filename: string | null }> {
    const res = await fetch(apiUrl(path), { headers: authHeaders() })
    if (res.status === 401) {
        onUnauthorized?.()
    }
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
