import { afterEach, describe, expect, it, vi } from 'vitest'
import {
    apiUrl,
    ApiError,
    configureApiClient,
    del,
    get,
    getRoot,
    post,
    setUnauthorizedHandler
} from './http'

function jsonResponse(body: unknown, status: number): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' }
    })
}

describe('http client', () => {
    afterEach(() => {
        configureApiClient({})
        setUnauthorizedHandler(null)
    })

    it('resolves with undefined on a 200 response with an empty body', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 200 })))

        await expect(getRoot<void>('/healthz')).resolves.toBeUndefined()
    })

    it('prefixes every request with the configured base URL', () => {
        configureApiClient({ baseUrl: 'https://geoapi.example.com/' })

        expect(apiUrl('/tables/t1')).toBe('https://geoapi.example.com/api/v1/tables/t1')
    })

    it('resolves with the parsed body on a 2xx response', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ tableId: 't1' }, 200)))

        await expect(get<{ tableId: string }>('/tables/t1')).resolves.toEqual({ tableId: 't1' })
    })

    it('resolves with undefined on a 204 response', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))

        await expect(del<void>('/tables/t1/rows/pk-1')).resolves.toBeUndefined()
    })

    it("rejects with an ApiError carrying the server's error body on failure", async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(
                jsonResponse(
                    {
                        error: 'not_found',
                        message: 'table t1 not found',
                        code: 404,
                        requestId: 'req-1'
                    },
                    404
                )
            )
        )

        await expect(get('/tables/unknown')).rejects.toMatchObject({
            message: 'table t1 not found',
            code: 404,
            requestId: 'req-1'
        })
    })

    it("falls back to the status text when the error body isn't JSON", async () => {
        vi.stubGlobal(
            'fetch',
            vi
                .fn()
                .mockResolvedValue(
                    new Response('boom', { status: 500, statusText: 'Internal Server Error' })
                )
        )

        await expect(get('/tables')).rejects.toBeInstanceOf(ApiError)
    })

    it('sends a JSON content-type and body on post', async () => {
        const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ok: true }, 201))
        vi.stubGlobal('fetch', fetchMock)

        await post('/tables/t1/derive', { query: 'SELECT 1' })

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/v1/tables/t1/derive',
            expect.objectContaining({
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ query: 'SELECT 1' })
            })
        )
    })

    it('prefixes every request with /api/v1', () => {
        expect(apiUrl('/tables/t1')).toBe('/api/v1/tables/t1')
    })

    it('carries the session cookie on every request, same-origin only', async () => {
        const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ tableId: 't1' }, 200))
        vi.stubGlobal('fetch', fetchMock)

        await get('/tables/t1')

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/v1/tables/t1',
            expect.objectContaining({ credentials: 'same-origin' })
        )
    })

    it('calls the registered handler on a 401, and still throws the ApiError', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(
                jsonResponse(
                    {
                        error: 'unauthorized',
                        message: 'no token',
                        code: 401,
                        requestId: 'req-2'
                    },
                    401
                )
            )
        )
        const handler = vi.fn()
        setUnauthorizedHandler(handler)

        await expect(get('/tables/t1')).rejects.toBeInstanceOf(ApiError)
        expect(handler).toHaveBeenCalledOnce()
    })
})
