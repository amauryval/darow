import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import * as api from '@/darow/api/darow'
import { useSources } from '@/darow/composables/useSources'

vi.mock('@/darow/api/darow', () => ({
    listSources: vi.fn(),
    listTables: vi.fn(),
    uploadSource: vi.fn(),
    deleteSource: vi.fn(),
    sourceLayers: vi.fn(),
    loadSource: vi.fn()
}))

vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: () => Promise.resolve(true) })
}))

const source = { name: 'places.geojson', size: 10, modifiedAt: '2026-01-01T00:00:00Z' }

beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(api.listSources).mockResolvedValue([source])
    vi.mocked(api.listTables).mockResolvedValue([])
})

describe('useSources', () => {
    it('lists the sources', async () => {
        const { sources, load } = useSources()

        await load()

        expect(sources.value).toEqual([source])
    })

    it('uploads a file, its progress cleared once sent', async () => {
        const { progress, send } = useSources()

        expect(await send(new File(['{}'], 'places.geojson'))).toBe(true)

        expect(api.uploadSource).toHaveBeenCalled()
        expect(progress.value).toBeNull()
    })

    it('loads a source into a table, then reloads the tables', async () => {
        const { loadInto } = useSources()
        const request = { table: 'places', layer: '', srid: 0 }

        expect(await loadInto('places.geojson', request)).toBe(true)

        expect(api.loadSource).toHaveBeenCalledWith('places.geojson', request)
        expect(api.listTables).toHaveBeenCalled()
    })

    it('reads the layers of a source', async () => {
        vi.mocked(api.sourceLayers).mockResolvedValue(['a', 'b'])
        const { layers } = useSources()

        expect(await layers('places.gpkg')).toEqual(['a', 'b'])
    })

    it('deletes a source', async () => {
        const { remove } = useSources()

        expect(await remove('places.geojson')).toBe(true)

        expect(api.deleteSource).toHaveBeenCalledWith('places.geojson')
    })
})
