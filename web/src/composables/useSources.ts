import { ref } from 'vue'
import type { SortingState } from '@tanstack/vue-table'
import {
    deleteSource,
    getSourceMetadata,
    listSources,
    loadSource,
    refreshSources,
    uploadSource
} from '../client/sources'
import type { Source } from '../client/types'
import { errorMessage } from '../client/http'
import { usePaginatedList } from './usePaginatedList'
import { useSettings } from './useSettings'
import { useToast } from './useToast'

// Display column ids that don't match the API's Source field names 1:1
// ("size" formats sizeBytes+layerCount, "status" formats loaded).
const sourceSortColumnMap: Record<string, string> = { size: 'sizeBytes', status: 'loaded' }

export function layerKey(source: Source, layerName: string): string {
    return `${source.name}::${layerName}`
}

// workDir source list, upload, per-layer loading (with EPSG override) and
// deletion. Built on usePaginatedList for the paging/sorting/scroll-sentinel
// triplet (TODO-F4).
export function useSources(initialSorting?: SortingState) {
    const toast = useToast()
    const settings = useSettings()

    // Capped at maxListLimit: a value the operator has lowered below this
    // default from /settings would otherwise make the list's own first
    // fetch fail against the server's own cap.
    const limit = Math.min(50, settings.current.value?.maxListLimit ?? 50)
    const list = usePaginatedList<Source>({
        limit,
        initialSorting,
        sortColumnMap: sourceSortColumnMap,
        fetchPage: (offset, limit, sort) =>
            listSources(offset, limit, sort).then((page) => ({
                items: page.sources,
                total: page.total
            })),
        describeError: (e) => errorMessage(e, 'Impossible de charger les sources')
    })

    // Distinct from reload(): re-scans workDir for new/removed files instead
    // of just re-fetching the already-known list.
    async function refresh() {
        try {
            const page = await refreshSources(0, limit, list.sortQuery())
            list.replace({ items: page.sources, total: page.total })
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de rafraîchir les sources'))
        }
    }

    // --- Upload ---
    const uploading = ref(false)

    // Resolves to { tableId } when the file was loaded straight away, or null
    // for a multi-layer GeoPackage (no layer auto-selected — the caller finds
    // it back in the reloaded list to let the user pick a layer). Rethrows on
    // failure so the caller can word the error with the file name.
    async function upload(file: File): Promise<{ tableId: string } | null> {
        uploading.value = true
        try {
            const result = await uploadSource(file)
            if ('tableId' in result && result.tableId !== undefined)
                return { tableId: result.tableId }
            await list.reload()
            return null
        } finally {
            uploading.value = false
        }
    }

    // --- Load (EPSG detection + confirmation) ---
    const epsgEditingKey = ref<string | null>(null)
    const epsgInput = ref('')
    const epsgDetecting = ref(false)
    // Whether the source being loaded has a geometry to project. Until the
    // detection answers, the question is assumed to apply — asking and finding
    // out it did not costs nothing, loading without asking cannot be undone.
    const epsgApplies = ref(true)
    const loadingKey = ref<string | null>(null)

    async function startLoad(source: Source, layerName?: string) {
        const key = layerName ? layerKey(source, layerName) : source.name
        epsgEditingKey.value = key
        epsgInput.value = ''
        epsgApplies.value = true
        epsgDetecting.value = true
        try {
            const crs = await getSourceMetadata(source.name, layerName)
            if (epsgEditingKey.value === key) {
                epsgInput.value = crs.srid ? String(crs.srid) : ''
                epsgApplies.value = crs.hasGeometry
            }
        } catch (e) {
            toast.error(
                errorMessage(e, `Impossible de détecter le système de coordonnées de ${key}`)
            )
        } finally {
            epsgDetecting.value = false
        }
        // Nothing to project, nothing to ask: the click that opened this was the
        // *Charger* the user meant, and a confirmation of an empty question is a
        // step that only asks them to click again.
        if (epsgEditingKey.value === key && !epsgApplies.value) await confirmLoad(source, layerName)
    }

    function cancelLoad() {
        epsgEditingKey.value = null
        epsgInput.value = ''
        epsgApplies.value = true
    }

    async function confirmLoad(source: Source, layerName?: string) {
        const key = layerName ? layerKey(source, layerName) : source.name
        loadingKey.value = key
        try {
            await loadSource(source.name, layerName, Number(epsgInput.value))
            epsgEditingKey.value = null
            await list.reload()
            toast.success(`${key} chargé.`)
        } catch (e) {
            toast.error(errorMessage(e, `Impossible de charger ${key}`))
        } finally {
            loadingKey.value = null
        }
    }

    // --- Delete ---
    const deletingName = ref<string | null>(null)

    async function remove(name: string) {
        deletingName.value = name
        try {
            await deleteSource(name)
            await list.reload()
            toast.success(`${name} supprimé.`)
        } catch (e) {
            toast.error(errorMessage(e, `Impossible de supprimer ${name}`))
        } finally {
            deletingName.value = null
        }
    }

    return {
        sources: list.items,
        total: list.total,
        loading: list.loading,
        loadedOnce: list.loadedOnce,
        error: list.error,
        retry: list.retry,
        sorting: list.sorting,
        sentinel: list.sentinel,
        setSorting: list.setSorting,
        reload: list.reload,
        refresh,
        uploading,
        upload,
        epsgEditingKey,
        epsgInput,
        epsgDetecting,
        epsgApplies,
        loadingKey,
        startLoad,
        cancelLoad,
        confirmLoad,
        deletingName,
        remove
    }
}
