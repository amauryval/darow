import { computed, ref, type Ref } from 'vue'
import { deriveTableFromTable } from '../client/tables'
import type { ColumnSchema, DeriveSpec } from '../client/types'
import { useAsyncAction } from './useAsyncAction'
import { useComputedColumnList } from './useComputedColumnList'
import { coerceByType } from '../utils/columnTypes'
import { pruneActive, type DraftNode } from '../utils/filterTree'
import { toggleInList } from '../utils/lists'

// The guided derivation from the table being viewed: a spec the server turns
// into SQL. Free-SQL derivation is `useDeriveFromQuery` — a top-level route
// scoped to no table, opened from the catalogue rather than one table's page.
//
// The guided draft lives here rather than in the form component: pruning the
// condition tree and coercing each value to the column's JSON type are
// business rules, not rendering (§0.2).
export function useDeriveTable(tableId: Ref<string>, columns: Ref<ColumnSchema[]>) {
    const { isLoading, error, run } = useAsyncAction()

    // Guided draft. Empty selections mean "everything": the server reads an
    // empty column list as the whole table, so nothing has to be preselected
    // for the default derivation to be the obvious one.
    const selectedColumns = ref<string[]>([])
    const {
        items: computedColumns,
        add: addComputed,
        update: updateComputed,
        remove: removeComputed,
        normalized: normalizedComputed,
        incomplete: incompleteComputed
    } = useComputedColumnList()
    const conditions = ref<DraftNode[]>([])
    const conditionsBool = ref<'AND' | 'OR'>('AND')

    const columnType = (name: string) => columns.value.find((c) => c.name === name)?.type ?? ''
    const coerce = (column: string, value: string) => coerceByType(columnType(column), value)

    const spec = computed<Omit<DeriveSpec, 'newTableName'>>(() => ({
        columns: selectedColumns.value,
        computed: normalizedComputed.value,
        conditions: pruneActive(conditions.value, coerce)
    }))

    function toggleColumn(name: string) {
        selectedColumns.value = toggleInList(selectedColumns.value, name)
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
