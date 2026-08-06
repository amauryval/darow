import { ref, type Ref } from 'vue'
import { addRows, deleteRow, getRowGeometry, listRows } from '../client/tables'
import type { Row } from '../client/types'
import { errorMessage } from '../client/http'
import { cellValue } from '../utils/rows'
import { usePaginatedList } from './usePaginatedList'
import { useSettings } from './useSettings'
import { useToast } from './useToast'

// A table's rows: the current (unfiltered) page, server-side sorting,
// insertion, duplication and deletion. Built on usePaginatedList for the
// paging/sorting/scroll-sentinel triplet rather than reimplementing it a
// fourth time (TODO-F4). onMutated is called after an add/delete, since
// that changes the table's rowCount — the caller owns table metadata
// (useTableMetadata) and reloads it.
export function useTableRows(
    tableId: Ref<string>,
    rowColumns: Ref<string[]>,
    geometryColumn: Ref<string | undefined>,
    onMutated: () => void
) {
    const toast = useToast()
    const settings = useSettings()

    // Capped at maxRowLimit, for the same reason useSources caps its own
    // default against maxListLimit: a lowered cap must not make the grid's
    // first page request fail against the server's own limit.
    const pageSize = Math.min(20, settings.current.value?.maxRowLimit ?? 20)

    const list = usePaginatedList<Row>({
        limit: pageSize,
        key: tableId,
        fetchPage: (offset, limit, sort) =>
            listRows(tableId.value, offset, limit, sort).then((page) => ({
                items: page.rows,
                total: page.total
            })),
        describeError: (e) => errorMessage(e, 'Impossible de charger les lignes')
    })

    // The physical column is named "__pkey", but Row objects key it as
    // "pkey" (router.rowToJSON pulls it from storage.Row's dedicated Pkey
    // field rather than its Values map) — map the two back together for
    // display rather than showing a blank cell.
    //
    // Retyping a cell is not here: an edit session holds its attribute drafts
    // whole (useRowEdits) and writes them with its own *Enregistrer*, next to
    // the geometries. What stays is what a single click does on the spot —
    // duplicating a row or deleting one. Creating one is not here either: a
    // new row is a draft of the session (useRowEdits) until it is saved.
    const deletingPkey = ref<string | null>(null)

    async function deleteOneRow(pkey: string) {
        deletingPkey.value = pkey
        try {
            await deleteRow(tableId.value, pkey)
            await list.reload()
            onMutated()
            toast.success('Ligne supprimée.')
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de supprimer la ligne'))
        } finally {
            deletingPkey.value = null
        }
    }

    // --- Duplicate row ---
    const duplicatingPkey = ref<string | null>(null)

    // A copy of the row under a new primary key: its attributes are taken from
    // the row already on screen, but its geometry has to be read for it. The
    // geometry column is deliberately absent from every row listing (LIM-F3),
    // and a geo row duplicated without it would land empty — a row that exists
    // in the grid and nowhere on the map.
    async function duplicateRow(row: Row) {
        duplicatingPkey.value = row.pkey
        try {
            const values: Record<string, unknown> = {}
            for (const col of rowColumns.value) values[col] = cellValue(row, col)
            const column = geometryColumn.value
            if (column) values[column] = (await getRowGeometry(tableId.value, row.pkey)).geometry
            await addRows(tableId.value, [values])
            await list.reload()
            onMutated()
            toast.success('Ligne dupliquée.')
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de dupliquer la ligne'))
        } finally {
            duplicatingPkey.value = null
        }
    }

    return {
        rows: list.items,
        total: list.total,
        pageSize,
        loading: list.loading,
        loadedOnce: list.loadedOnce,
        error: list.error,
        retry: list.retry,
        sorting: list.sorting,
        sentinel: list.sentinel,
        setSorting: list.setSorting,
        deletingPkey,
        deleteRow: deleteOneRow,
        duplicatingPkey,
        duplicateRow
    }
}
