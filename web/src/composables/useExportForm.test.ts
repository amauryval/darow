import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useExportForm } from './useExportForm'
import type { Metadata } from '../client/types'

function table(isGeo: boolean, srid = 4326): Metadata {
    return {
        tableId: 't1',
        physicalName: 't1',
        tableName: 'routes',
        isGeo,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        rowCount: 3,
        srid
    }
}

function setup(isGeo = true, srid = 4326) {
    const onExport = vi.fn().mockResolvedValue(undefined)
    return { onExport, ...useExportForm(ref(table(isGeo, srid)), onExport) }
}

describe('useExportForm', () => {
    it('defaults a geo table to GeoJSON and a plain one to CSV', () => {
        expect(setup(true).format.value).toBe('geojson')
        expect(setup(false, 0).format.value).toBe('csv')
    })

    it('pre-fills the table’s own CRS, which is also the server’s fallback', () => {
        expect(setup(true, 2154).srid.value).toBe('2154')
        expect(setup(false, 0).srid.value).toBe('')
    })

    it('reprojects nothing for CSV, which carries no geometry', () => {
        const { format, carriesGeometry } = setup(true)

        expect(carriesGeometry.value).toBe(true)
        format.value = 'csv'
        expect(carriesGeometry.value).toBe(false)
    })

    it('reads an EPSG code, treating an empty field as the server’s fallback', () => {
        const { srid, parsedSrid } = setup()

        expect(parsedSrid.value).toBe(4326)
        srid.value = '   '
        expect(parsedSrid.value).toBeNull()
    })

    it('warns that a geometry column turns a Parquet into a GeoParquet', () => {
        const { format, formatHint } = setup(true)

        format.value = 'parquet'

        expect(formatHint.value).toBe('La géométrie fait de ce Parquet un GeoParquet.')
    })

    it('warns that a table with no geometry cannot be served as GeoJSON', () => {
        const { format, formatHint } = setup(false, 0)

        format.value = 'geojson'

        expect(formatHint.value).toBe('Cette table n’a pas de géométrie : le GeoJSON sera refusé.')
    })

    it('states no hint for a plain combination', () => {
        expect(setup(true).formatHint.value).toBeNull()
    })

    it('exports the chosen format and CRS', async () => {
        const { srid, onExport, confirm } = setup(true)
        srid.value = '2154'

        await expect(confirm()).resolves.toBe(true)

        expect(onExport).toHaveBeenCalledWith('geojson', 2154)
    })

    it('passes no CRS on a format carrying no geometry', async () => {
        const { format, onExport, confirm } = setup(true)
        format.value = 'csv'

        await confirm()

        expect(onExport).toHaveBeenCalledWith('csv', null)
    })

    it('blocks on an EPSG code that is not a positive integer, before writing', async () => {
        const { srid, error, onExport, confirm } = setup(true)
        srid.value = 'lambert'

        await expect(confirm()).resolves.toBe(false)

        expect(error.value).toBe('Le code EPSG doit être un entier positif.')
        expect(onExport).not.toHaveBeenCalled()
    })

    it('states a refused export in French', async () => {
        const onExport = vi.fn().mockRejectedValue(new Error('trop volumineux'))
        const { error, confirm } = useExportForm(ref(table(true)), onExport)

        await expect(confirm()).resolves.toBe(false)

        expect(error.value).toBe("Impossible d'exporter la table : trop volumineux")
    })
})
