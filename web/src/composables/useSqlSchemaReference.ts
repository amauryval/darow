import { ref } from 'vue'
import { getSchema, listTables } from '../client/tables'
import { errorMessage } from '../client/http'
import { LIST_PAGE_SIZE } from '../utils/pageSizes'
import { usePaginatedList } from './usePaginatedList'
import type { ColumnSchema, Metadata } from '../client/types'

// Backs the free-SQL derivation's reference panel: the registered tables,
// infinite-scrolled like TablesView, and each one's columns loaded only once
// expanded and cached from then on — the same restraint
// useTableSettingsOverrides applies to a table-wide sweep, since a
// deployment can carry far more tables than a query ever names.
export function useSqlSchemaReference() {
    const {
        items: tables,
        total,
        loading,
        error,
        loadedOnce,
        sentinel
    } = usePaginatedList<Metadata>({
        limit: LIST_PAGE_SIZE,
        fetchPage: (offset, limit) =>
            listTables(offset, limit).then((page) => ({ items: page.tables, total: page.total })),
        describeError: (e) => errorMessage(e, 'Impossible de charger les tables')
    })

    const expanded = ref<string | null>(null)
    const columnsByTable = ref<Record<string, ColumnSchema[]>>({})
    const loadingColumns = ref<string | null>(null)

    async function toggle(tableId: string) {
        expanded.value = expanded.value === tableId ? null : tableId
        if (expanded.value === null || columnsByTable.value[tableId]) return

        loadingColumns.value = tableId
        try {
            const { columns } = await getSchema(tableId)
            columnsByTable.value = { ...columnsByTable.value, [tableId]: columns }
        } catch {
            // Best-effort: the panel is a convenience over the textarea, not
            // a form — a failed column load just leaves that table expanded
            // with nothing under it, the query itself is unaffected.
        } finally {
            loadingColumns.value = null
        }
    }

    return {
        tables,
        total,
        loading,
        error,
        loadedOnce,
        sentinel,
        expanded,
        columnsByTable,
        loadingColumns,
        toggle
    }
}
