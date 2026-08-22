import { beforeEach, describe, expect, it, vi } from 'vitest'
import { layerKey, useSources } from './useSources'
import { flush, stubIntersectionObserver, withSetup } from '../test/fixtures'
import { useToast } from './useToast'
import {
    deleteSource,
    getSourceLimits,
    listSources,
    refreshSources,
    uploadSource
} from '../client/sources'
import type { Source } from '../client/types'

vi.mock('../client/sources', () => ({
    listSources: vi.fn(),
    refreshSources: vi.fn(),
    uploadSource: vi.fn(),
    deleteSource: vi.fn(),
    getSourceLimits: vi.fn()
}))

const list = vi.mocked(listSources)
const refreshWorkDir = vi.mocked(refreshSources)
const upload = vi.mocked(uploadSource)
const remove = vi.mocked(deleteSource)
const limits = vi.mocked(getSourceLimits)

function source(name: string): Source {
    return { name, ext: '.geojson', sizeBytes: 1024, modTime: '2026-01-01T00:00:00Z' }
}

async function setup() {
    list.mockResolvedValue({ sources: [source('routes.geojson')], total: 1, limit: 50, offset: 0 })
    limits.mockResolvedValue({ maxSizeBytes: 1024, allowedExtensions: ['.geojson'] })
    const { result } = withSetup(() => useSources())
    await flush()
    return result
}

describe('layerKey', () => {
    it('identifies a layer by its file and its own name', () => {
        expect(layerKey(source('atlas.gpkg'), 'routes')).toBe('atlas.gpkg::routes')
    })
})

describe('useSources', () => {
    beforeEach(() => {
        stubIntersectionObserver()
        useToast().toasts.value = []
    })

    it('loads the workDir listing', async () => {
        const { sources, total } = await setup()

        expect(list).toHaveBeenCalledWith(0, 50, '')
        expect(sources.value).toEqual([source('routes.geojson')])
        expect(total.value).toBe(1)
    })

    it('sorts on the API field name behind a display column', async () => {
        const { setSorting } = await setup()

        setSorting([{ id: 'size', desc: true }])
        await flush()

        expect(list).toHaveBeenLastCalledWith(0, 50, '-sizeBytes')
    })

    it('reads the deployment’s own cap and whitelist', async () => {
        const { limits: read } = await setup()

        expect(read.value).toEqual({ maxSizeBytes: 1024, allowedExtensions: ['.geojson'] })
    })

    it('does without the hint when the cap cannot be read', async () => {
        list.mockResolvedValue({ sources: [], total: 0, limit: 50, offset: 0 })
        limits.mockRejectedValue(new Error('boom'))

        const { result } = withSetup(() => useSources())
        await flush()

        expect(result.limits.value).toBeNull()
    })

    it('re-scans workDir rather than re-fetching the known list', async () => {
        const { sources, refresh } = await setup()
        refreshWorkDir.mockResolvedValue({
            sources: [source('nouveau.geojson')],
            total: 1,
            limit: 50,
            offset: 0
        })

        await refresh()

        expect(refreshWorkDir).toHaveBeenCalledWith(0, 50, '')
        expect(sources.value).toEqual([source('nouveau.geojson')])
    })

    it('toasts a failed re-scan', async () => {
        const { refresh } = await setup()
        const { toasts } = useToast()
        refreshWorkDir.mockRejectedValue(new Error('boom'))

        await refresh()

        expect(toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de rafraîchir les sources : boom'
        })
    })

    it('reports the upload’s progress, then reloads the listing', async () => {
        const { upload: send, uploading, uploadProgress } = await setup()
        const seen: (number | null)[] = []
        upload.mockImplementation(async (_file, onProgress) => {
            expect(uploading.value).toBe(true)
            onProgress(0.5)
            seen.push(uploadProgress.value)
            return {}
        })

        await send(new File(['x'], 'routes.geojson'))

        expect(seen).toEqual([0.5])
        expect(uploading.value).toBe(false)
        expect(uploadProgress.value).toBeNull()
        expect(list).toHaveBeenCalledTimes(2)
    })

    it('rethrows a refused upload, so the caller words it with the file name', async () => {
        const { upload: send, uploading } = await setup()
        upload.mockRejectedValue(new Error('trop volumineux'))

        await expect(send(new File(['x'], 'routes.geojson'))).rejects.toThrow('trop volumineux')
        expect(uploading.value).toBe(false)
    })

    it('deletes a file, reloads and confirms with a toast', async () => {
        const { remove: removeOne, deletingName } = await setup()
        const { toasts } = useToast()
        remove.mockResolvedValue({ id: 'routes.geojson' })

        await removeOne('routes.geojson')

        expect(deletingName.value).toBeNull()
        expect(list).toHaveBeenCalledTimes(2)
        expect(toasts.value[0].message).toBe('routes.geojson supprimé du répertoire de travail.')
    })

    it('toasts a refused deletion, naming the file', async () => {
        const { remove: removeOne } = await setup()
        const { toasts } = useToast()
        remove.mockRejectedValue(new Error('boom'))

        await removeOne('routes.geojson')

        expect(toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de supprimer routes.geojson : boom'
        })
    })
})
