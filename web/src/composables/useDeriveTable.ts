import { computed, ref, type Ref } from 'vue'
import { deriveTableFromTable } from '../client/tables'
import { errorMessage } from '../client/http'
import type { ColumnSchema, ComputedColumn, DeriveSpec } from '../client/types'
import { coerceByType } from '../utils/columnTypes'
import { pruneActive, type DraftNode } from '../utils/filterTree'

// The guided derivation from the table being viewed: a spec the server turns
// into SQL. Free-SQL derivation is `useDeriveFromQuery` — a top-level route
// scoped to no table, opened from the catalogue rather than one table's page
// (TODO-F38).
//
// The guided draft lives here rather than in the form component: pruning the
// condition tree and coercing each value to the column's JSON type are
// business rules, not rendering (§0.2).
export function useDeriveTable(tableId: Ref<string>, columns: Ref<ColumnSchema[]>) {
    const isLoading = ref(false)
    const error = ref('')

    // Guided draft. Empty selections mean "everything": the server reads an
    // empty column list as the whole table, so nothing has to be preselected
    // for the default derivation to be the obvious one.
    const selectedColumns = ref<string[]>([])
    const computedColumns = ref<ComputedColumn[]>([])
    const conditions = ref<DraftNode[]>([])
    const conditionsBool = ref<'AND' | 'OR'>('AND')

    const columnType = (name: string) => columns.value.find((c) => c.name === name)?.type ?? ''
    const coerce = (column: string, value: string) => coerceByType(columnType(column), value)

    const spec = computed<Omit<DeriveSpec, 'newTableName'>>(() => ({
        columns: selectedColumns.value,
        computed: computedColumns.value
            .filter((c) => c.alias.trim() !== '' || c.expression.trim() !== '')
            .map((c) => ({ alias: c.alias.trim(), expression: c.expression.trim() })),
        conditions: pruneActive(conditions.value, coerce)
    }))

    // A computed column half filled in is rejected by the server, so it is
    // blocked here too — with the difference that here it can be pointed at.
    const incompleteComputed = computed(() =>
        (spec.value.computed ?? []).some((c) => c.alias === '' || c.expression === '')
    )

    function toggleColumn(name: string) {
        selectedColumns.value = selectedColumns.value.includes(name)
            ? selectedColumns.value.filter((c) => c !== name)
            : [...selectedColumns.value, name]
    }

    function addComputed() {
        computedColumns.value = [...computedColumns.value, { alias: '', expression: '' }]
    }

    function updateComputed(index: number, patch: Partial<ComputedColumn>) {
        computedColumns.value = computedColumns.value.map((c, i) =>
            i === index ? { ...c, ...patch } : c
        )
    }

    function removeComputed(index: number) {
        computedColumns.value = computedColumns.value.filter((_, i) => i !== index)
    }

    async function run<T>(action: () => Promise<T>, fallback: string): Promise<T | null> {
        isLoading.value = true
        error.value = ''
        try {
            return await action()
        } catch (e) {
            error.value = errorMessage(e, fallback)
            return null
        } finally {
            isLoading.value = false
        }
    }

    function submitSpec(newTableName: string) {
        return run(
            () => deriveTableFromTable(tableId.value, { newTableName, ...spec.value }),
            'Impossible de dériver la table'
        )
    }

    return {
        isLoading,
        error,
        selectedColumns,
        computedColumns,
        conditions,
        conditionsBool,
        incompleteComputed,
        toggleColumn,
        addComputed,
        updateComputed,
        removeComputed,
        submitSpec
    }
}
