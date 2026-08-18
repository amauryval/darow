import type { Ref } from 'vue'
import type { SortingState } from '@tanstack/vue-table'
import {
    addRows,
    deleteRows as deleteRowsRequest,
    getRowGeometry,
    listRows
} from '../client/tables'
import type { Row } from '../client/types'
import { errorMessage } from '../client/http'
import { inBatches } from '../utils/batch'
import { cellValue } from '../utils/rows'
import { useAsyncAction } from './useAsyncAction'
import { usePaginatedList } from './usePaginatedList'
import { useToast } from './useToast'

// A table's rows: the current (unfiltered) page, server-side sorting,
// insertion, duplication and deletion. Built on usePaginatedList for the
// paging/sorting/scroll-sentinel triplet rather than reimplementing it a
// fourth time. onMutated is called after an add/delete, since
// that changes the table's rowCount — the caller owns table metadata
// (useTableMetadata) and reloads it. rowLimit is the table's effective row
// limit (useTableSettings.rowLimit), never a size of this composable's
// own — a table with a raised limit must not stay capped by the client.
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
    const { isLoading: deleting, error: deleteError, run: runDelete } = useAsyncAction()

    // One request for the whole selection (`DELETE /tables/{tableId}/rows`,
    // a single storage-level write). The grid then drops exactly what was
    // asked for rather than re-fetching page 1 — the list already knows
    // which rows disappeared, and a refetch emptied it for a round-trip
    // after every deletion (§8.1, §8.2).
    async function deleteRows(pkeys: string[]) {
        if (pkeys.length === 0) return
        const result = await runDelete(
            () => deleteRowsRequest(tableId.value, pkeys),
            'Impossible de supprimer la sélection'
        )
        if (result === null) {
            toast.error(deleteError.value)
            return
        }
        const gone = new Set(pkeys)
        list.drop((row) => gone.has(row.pkey))
        onMutated()
        toast.success(pkeys.length > 1 ? `${pkeys.length} lignes supprimées.` : 'Ligne supprimée.')
    }

    const { isLoading: duplicating, error: duplicateError, run: runDuplicate } = useAsyncAction()

    // Copies under new primary keys: the attributes are taken from the rows
    // already on screen, but each geometry has to be read for it. The geometry
    // column is deliberately absent from every row listing (LIM-F3), and a geo
    // row duplicated without it would land empty — a row that exists in the
    // grid and nowhere on the map.
    async function duplicateRows(rows: Row[]) {
        if (rows.length === 0) return
        const copies = await runDuplicate(async () => {
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
            return copies
        }, 'Impossible de dupliquer la sélection')
        if (copies === null) {
            toast.error(duplicateError.value)
            return
        }
        toast.success(
            copies.length > 1 ? `${copies.length} lignes dupliquées.` : 'Ligne dupliquée.'
        )
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
