import { computed, ref, watch, type Ref } from 'vue'
import {
    addColumn,
    deleteTable,
    dropColumn,
    getSchema,
    getTable,
    renameColumn,
    renameTable
} from '../client/tables'
import type { ColumnSchema, Metadata } from '../client/types'
import { errorMessage } from '../client/http'
import { isHiddenColumn } from '../utils/rows'

// A table's metadata and schema, reloaded whenever tableId changes (vue-router
// reuses TableDetailView across :tableId navigations, so this must react to
// the ref rather than run once in onMounted). Mutating actions (rename,
// delete, addColumn, renameExistingColumn, removeColumn) do the network call
// and rethrow on failure — callers decide whether the error becomes a toast or an inline form
// message, since that varies by call site (règle 8.4). loadAll's own failure
// is different: it is the initial load, so it is exposed as loading/error
// (§0.18, §8.1) for the view to render a Skeleton/ErrorState instead of a
// toast over a blank page.
export function useTableMetadata(tableId: Ref<string>) {
    const table = ref<Metadata | null>(null)
    const columns = ref<ColumnSchema[]>([])
    const loading = ref(false)
    const loadedOnce = ref(false)
    const error = ref('')

    // Columns actually writable by AddRows/UpdateRow: everything but the
    // server-generated pkey and OGC_FID (GDAL's injected feature-id column,
    // kept in storage but never surfaced — see isHiddenColumn). The geometry
    // column (if any) stays in this list — InsertRow/UpdateRow accept WKT
    // for it — but it's excluded from GetRows/GetRowsWithFilter, so it never
    // has a value to display.
    const editableColumns = computed(() =>
        columns.value.map((c) => c.name).filter((name) => !isHiddenColumn(name))
    )
    const rowColumns = computed(() =>
        editableColumns.value.filter((name) => name !== table.value?.geometryColumn)
    )

    async function loadTable() {
        table.value = await getTable(tableId.value)
    }

    async function loadSchema() {
        const res = await getSchema(tableId.value)
        columns.value = res.columns
    }

    async function loadAll() {
        loading.value = true
        error.value = ''
        try {
            await Promise.all([loadTable(), loadSchema()])
            loadedOnce.value = true
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de charger la table')
        } finally {
            loading.value = false
        }
    }

    async function rename(name: string) {
        table.value = await renameTable(tableId.value, name)
    }

    function remove() {
        return deleteTable(tableId.value)
    }

    async function addNewColumn(name: string, type: string) {
        await addColumn(tableId.value, name, type)
        await loadSchema()
    }

    async function renameExistingColumn(name: string, newName: string) {
        await renameColumn(tableId.value, name, newName)
        await loadSchema()
    }

    async function removeColumn(name: string) {
        await dropColumn(tableId.value, name)
        await loadSchema()
    }

    watch(tableId, loadAll, { immediate: true })

    return {
        table,
        columns,
        editableColumns,
        rowColumns,
        loading,
        loadedOnce,
        error,
        retry: loadAll,
        loadTable,
        rename,
        remove,
        addNewColumn,
        renameExistingColumn,
        removeColumn
    }
}
