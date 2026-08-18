import { ref, type Ref } from 'vue'
import type { SortingState } from '@tanstack/vue-table'
import { addRows, deleteRow, getRowGeometry, listRows } from '../client/tables'
import type { Row } from '../client/types'
import { errorMessage } from '../client/http'
import { inBatches } from '../utils/batch'
import { cellValue } from '../utils/rows'
import { usePaginatedList } from './usePaginatedList'
import { useToast } from './useToast'

// The grid's page size before the table's effective row limit has loaded
// (TODO-F43) — used as the initial value of the rowLimit ref a caller
// passes to useTableRows, never recomputed here.
export const PAGE_SIZE = 50

// A table's rows: the current (unfiltered) page, server-side sorting,
// insertion, duplication and deletion. Built on usePaginatedList for the
// paging/sorting/scroll-sentinel triplet rather than reimplementing it a
// fourth time (TODO-F4). onMutated is called after an add/delete, since
// that changes the table's rowCount — the caller owns table metadata
// (useTableMetadata) and reloads it. rowLimit is the table's effective row
// limit (useTableSettings), PAGE_SIZE serving only until it has loaded — a
// table with a raised limit must not stay capped at the client's own
// constant.
export function useTableRows(
    tableId: Ref<string>,
    rowColumns: Ref<string[]>,
    geometryColumn: Ref<string | undefined>,
    onMutated: () => void,
    rowLimit: Ref<number>,
    initialSorting?: SortingState
) {
    const toast = useToast()

    const list = usePaginatedList<Row>({
        limit: rowLimit,
        key: tableId,
        initialSorting,
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
    // Duplicating and deleting act on the **selection**, never on one row from
    // its own cell: the grid already has a gesture that designates rows, and a
    // single row is a selection of one (§8.6). Retyping is not here either — a
    // session holds its attribute drafts whole (useRowEdits) and writes them
    // with its own *Enregistrer*.
    const deleting = ref(false)

    // No batch delete endpoint exists (LIM-F14): the rows go one request each,
    // in parallel batches. The grid then drops exactly what the server dropped
    // instead of re-fetching page 1 — the list already knows which rows
    // disappeared, and a refetch emptied it for a round-trip after every
    // deletion (§8.1, §8.2).
    async function deleteRows(pkeys: string[]) {
        if (pkeys.length === 0) return
        deleting.value = true
        try {
            const { done, reason } = await inBatches(pkeys, (pkey) =>
                deleteRow(tableId.value, pkey)
            )
            if (done.length > 0) {
                const gone = new Set(done)
                list.drop((row) => gone.has(row.pkey))
                onMutated()
            }
            if (reason !== null) {
                toast.error(errorMessage(reason, 'Impossible de supprimer la sélection'))
            } else {
                toast.success(
                    done.length > 1 ? `${done.length} lignes supprimées.` : 'Ligne supprimée.'
                )
            }
        } finally {
            deleting.value = false
        }
    }

    const duplicating = ref(false)

    // Copies under new primary keys: the attributes are taken from the rows
    // already on screen, but each geometry has to be read for it. The geometry
    // column is deliberately absent from every row listing (LIM-F3), and a geo
    // row duplicated without it would land empty — a row that exists in the
    // grid and nowhere on the map.
    async function duplicateRows(rows: Row[]) {
        if (rows.length === 0) return
        duplicating.value = true
        try {
            const geometries = new Map<string, unknown>()
            const column = geometryColumn.value
            if (column) {
                const { reason } = await inBatches(rows, async (row) => {
                    geometries.set(
                        row.pkey,
                        (await getRowGeometry(tableId.value, row.pkey)).geometry
                    )
                })
                if (reason !== null) throw reason
            }
            const copies = rows.map((row) => {
                const values: Record<string, unknown> = {}
                for (const col of rowColumns.value) values[col] = cellValue(row, col)
                if (column) values[column] = geometries.get(row.pkey)
                return values
            })
            await addRows(tableId.value, copies)
            // The copies land wherever the current sort puts them, so the page
            // is re-read — but through reloadSorted, which keeps the rows on
            // screen until the new ones arrive rather than flashing empty (§8.1).
            await list.reloadSorted()
            onMutated()
            toast.success(
                copies.length > 1 ? `${copies.length} lignes dupliquées.` : 'Ligne dupliquée.'
            )
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de dupliquer la sélection'))
        } finally {
            duplicating.value = false
        }
    }

    return {
        rows: list.items,
        total: list.total,
        pageSize: rowLimit,
        loading: list.loading,
        loadedOnce: list.loadedOnce,
        error: list.error,
        retry: list.retry,
        sorting: list.sorting,
        sentinel: list.sentinel,
        setSorting: list.setSorting,
        deleting,
        deleteRows,
        duplicating,
        duplicateRows
    }
}
