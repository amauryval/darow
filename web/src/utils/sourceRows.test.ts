import { describe, expect, it } from 'vitest'
import { formatSize, toSourceRows } from './sourceRows'
import type { Source } from '../client/types'

function source(name: string, layers?: string[]): Source {
    return { name, ext: '.gpkg', sizeBytes: 2048, modTime: '2026-01-01T00:00:00Z', layers }
}

describe('toSourceRows', () => {
    it('renders a single-layer file as one flat row', () => {
        expect(toSourceRows([source('routes.geojson')])).toEqual([
            { name: 'routes.geojson', sizeBytes: 2048, source: source('routes.geojson') }
        ])
    })

    it('expands a multi-layer file over its layers', () => {
        const gpkg = source('atlas.gpkg', ['routes', 'communes'])

        const [row] = toSourceRows([gpkg])

        expect(row.layerCount).toBe(2)
        expect(row.subRows).toEqual([
            { name: 'routes', source: gpkg, layerName: 'routes' },
            { name: 'communes', source: gpkg, layerName: 'communes' }
        ])
    })

    it('treats an empty layer list as a single-layer file', () => {
        const [row] = toSourceRows([source('vide.gpkg', [])])

        expect(row.subRows).toBeUndefined()
        expect(row.layerCount).toBeUndefined()
    })
})

describe('formatSize', () => {
    it('states bytes under a kilobyte', () => {
        expect(formatSize(512)).toBe('512 o')
    })

    it('states kilobytes under a megabyte', () => {
        expect(formatSize(1024)).toBe('1.0 Ko')
        expect(formatSize(1536)).toBe('1.5 Ko')
    })

    it('states megabytes past a megabyte', () => {
        expect(formatSize(1024 * 1024 * 3)).toBe('3.0 Mo')
    })
})
