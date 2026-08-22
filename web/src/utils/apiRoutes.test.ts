import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildTableApiRoutes } from './apiRoutes'
import { configureApiClient } from '../client/http'
import { LIST_PAGE_SIZE } from './pageSizes'

describe('buildTableApiRoutes', () => {
    // absoluteApiUrl falls back on the page's own origin when no base URL is
    // configured, which the node environment does not provide.
    beforeEach(() => vi.stubGlobal('window', { location: { origin: 'https://darow.test' } }))
    afterEach(() => {
        vi.unstubAllGlobals()
        configureApiClient({})
    })

    it('exposes the table, schema and row routes of a non-geo table', () => {
        const routes = buildTableApiRoutes('t1', false)

        expect(routes.map((r) => r.group)).toEqual(['Table', 'Table', 'Lignes', 'Lignes'])
        expect(routes.every((r) => r.method === 'GET')).toBe(true)
        expect(routes[0].url).toBe('https://darow.test/api/v1/tables/t1')
        expect(routes[2].url).toContain(`/tables/t1/rows?offset=0&limit=${LIST_PAGE_SIZE}&sort=`)
    })

    it('adds the geometry, tile and OGC routes of a geo table', () => {
        const routes = buildTableApiRoutes('t1', true)

        expect(routes.map((r) => r.url.split('/api/v1')[1])).toEqual([
            '/tables/t1',
            '/tables/t1/schema',
            `/tables/t1/rows?offset=0&limit=${LIST_PAGE_SIZE}&sort=`,
            '/tables/t1/rows/{pkey}',
            '/tables/t1/rows/{pkey}/geometry',
            '/tables/t1/tiles/{z}/{x}/{y}',
            '/ogc/collections/t1'
        ])
    })

    it('builds absolute URLs on the configured base', () => {
        configureApiClient({ baseUrl: 'https://geoapi.example.com/' })

        expect(buildTableApiRoutes('t1', false)[0].url).toBe(
            'https://geoapi.example.com/api/v1/tables/t1'
        )
    })
})
