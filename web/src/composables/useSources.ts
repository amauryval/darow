import { ref } from 'vue'
import type { SortingState } from '@tanstack/vue-table'
import { deleteSource, listSources, refreshSources, uploadSource } from '../client/sources'
import type { Source } from '../client/types'
import { errorMessage } from '../client/http'
import { usePaginatedList } from './usePaginatedList'
import { useToast } from './useToast'

// The list's own page size — deliberately independent of the operator's
// defaultListLimit setting, a REST API page size rather than a UI choice; see
// useTableRows' PAGE_SIZE for why.
const PAGE_SIZE = 50

// Display column ids that don't match the API's Source field names 1:1
// ("size" formats sizeBytes+layerCount).
const sourceSortColumnMap: Record<string, string> = { size: 'sizeBytes' }

export function layerKey(source: Source, layerName: string): string {
    return `${source.name}::${layerName}`
}

// workDir source list, refresh, upload and file deletion. Built on
// usePaginatedList for the paging/sorting/sentinel triplet; loading a source
// into a table is useSourceLoad's own flow.
export function useSources(initialSorting?: SortingState) {
    const toast = useToast()

    const list = usePaginatedList<Source>({
        limit: PAGE_SIZE,
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
            const page = await refreshSources(0, PAGE_SIZE, list.sortQuery())
            list.replace({ items: page.sources, total: page.total })
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de rafraîchir les sources'))
        }
    }

    // --- Upload ---
    const uploading = ref(false)

    // Writes the file into workDir; loading it is the user's next, explicit
    // step. Rethrows so the caller can word the error with the file name.
    async function upload(file: File): Promise<void> {
        uploading.value = true
        try {
            await uploadSource(file)
            await list.reload()
        } finally {
            uploading.value = false
        }
    }

    // --- Delete ---
    const deletingName = ref<string | null>(null)

    async function remove(name: string) {
        deletingName.value = name
        try {
            const deleted = await deleteSource(name)
            await list.reload()
            toast.success(`${deleted.id} supprimé du répertoire de travail.`)
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
        deletingName,
        remove
    }
}
