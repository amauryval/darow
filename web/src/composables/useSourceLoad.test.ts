import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useSourceLoad } from './useSourceLoad'
import { useToast } from './useToast'
import { getSourceMetadata, loadSource } from '../client/sources'
import type { Source } from '../client/types'

vi.mock('../client/sources', () => ({
    getSourceMetadata: vi.fn(),
    loadSource: vi.fn()
}))

const metadata = vi.mocked(getSourceMetadata)
const load = vi.mocked(loadSource)

const file: Source = {
    name: 'atlas.gpkg',
    ext: '.gpkg',
    sizeBytes: 1024,
    modTime: '2026-01-01T00:00:00Z'
}

describe('useSourceLoad', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        metadata.mockResolvedValue({ srid: 2154, authName: 'EPSG', hasGeometry: true })
        load.mockResolvedValue({ tableId: 't1' })
    })

    it('opens the form on the source and fills in the CRS it declares', async () => {
        const { loadTarget, detectedSrid, epsgApplies, startLoad } = useSourceLoad()

        await startLoad(file, 'routes')

        expect(metadata).toHaveBeenCalledWith('atlas.gpkg', 'routes')
        expect(loadTarget.value).toEqual({ source: file, layerName: 'routes' })
        expect(detectedSrid.value).toBe('2154')
        expect(epsgApplies.value).toBe(true)
    })

    it('drops the CRS question for a source with no geometry', async () => {
        metadata.mockResolvedValue({ srid: null, authName: '', hasGeometry: false })
        const { detectedSrid, epsgApplies, startLoad } = useSourceLoad()

        await startLoad(file)

        expect(detectedSrid.value).toBe('')
        expect(epsgApplies.value).toBe(false)
    })

    it('keeps the form open and toasts when the detection fails', async () => {
        metadata.mockRejectedValue(new Error('illisible'))
        const { loadTarget, epsgApplies, startLoad } = useSourceLoad()
        const { toasts } = useToast()

        await startLoad(file, 'routes')

        expect(loadTarget.value).not.toBeNull()
        expect(epsgApplies.value).toBe(true)
        expect(toasts.value[0]).toMatchObject({
            kind: 'error',
            message:
                'Impossible de détecter le système de coordonnées de atlas.gpkg::routes : illisible'
        })
    })

    it('never fills the form with a detection the user has moved past', async () => {
        metadata.mockResolvedValueOnce({ srid: 4326, authName: 'EPSG', hasGeometry: true })
        const { detectedSrid, startLoad } = useSourceLoad()
        const first = startLoad(file, 'routes')

        metadata.mockResolvedValue({ srid: 2154, authName: 'EPSG', hasGeometry: true })
        await startLoad(file, 'communes')
        await first

        expect(detectedSrid.value).toBe('2154')
    })

    it('closes the form on cancel', async () => {
        const { loadTarget, detectedSrid, cancelLoad, startLoad } = useSourceLoad()
        await startLoad(file, 'routes')

        cancelLoad()

        expect(loadTarget.value).toBeNull()
        expect(detectedSrid.value).toBe('')
    })

    it('creates the table, carrying the layer the form is open on', async () => {
        const { startLoad, confirmLoad, loadTarget } = useSourceLoad()
        const { toasts } = useToast()
        await startLoad(file, 'routes')

        await expect(confirmLoad({ srid: 2154, tableName: 'routes' })).resolves.toBe(true)

        expect(load).toHaveBeenCalledWith('atlas.gpkg', {
            srid: 2154,
            tableName: 'routes',
            layer: 'routes'
        })
        expect(loadTarget.value).toBeNull()
        expect(toasts.value[0].message).toBe('atlas.gpkg::routes chargé — la table a été créée.')
    })

    it('says a reload updated the table rather than created one', async () => {
        const { startLoad, confirmLoad } = useSourceLoad()
        const { toasts } = useToast()
        await startLoad(file)

        await confirmLoad({ replaceTableId: 't1' })

        expect(toasts.value[0].message).toBe('atlas.gpkg rechargé — la table a été mise à jour.')
    })

    it('keeps the form open and toasts when the load is refused', async () => {
        load.mockRejectedValue(new Error('CRS manquant'))
        const { startLoad, confirmLoad, loadTarget } = useSourceLoad()
        const { toasts } = useToast()
        await startLoad(file)

        await expect(confirmLoad({})).resolves.toBe(false)

        expect(loadTarget.value).not.toBeNull()
        expect(toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de charger atlas.gpkg : CRS manquant'
        })
    })

    it('writes nothing when no form is open', async () => {
        const { confirmLoad } = useSourceLoad()

        await expect(confirmLoad({})).resolves.toBe(false)
        expect(load).not.toHaveBeenCalled()
    })
})
