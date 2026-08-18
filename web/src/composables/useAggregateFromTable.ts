import { computed, ref, type Ref } from 'vue'
import { aggregateTableFromTable } from '../client/tables'
import type { AggregateSpec, ColumnSchema } from '../client/types'
import { useAsyncAction } from './useAsyncAction'
import { useComputedColumnList } from './useComputedColumnList'
import { coerceByType } from '../utils/columnTypes'
import { pruneActive, type DraftNode } from '../utils/filterTree'
import { toggleInList } from '../utils/lists'

// The guided aggregation from the table being viewed: GROUP BY columns plus
// aggregate expressions — the aggregation counterpart of useDeriveTable's
// guided mode, but its own composable rather than a third mode there
// (TODO-F37). The two share no state: a column is either carried over
// (row-wise) or grouped/aggregated (here), never both, and a shared
// column-selection concept would let a column be "selected" without being
// either — which GROUP BY refuses as a query error, not a default.
export function useAggregateFromTable(tableId: Ref<string>, columns: Ref<ColumnSchema[]>) {
    const { isLoading, error, run } = useAsyncAction()

    const groupBy = ref<string[]>([])
    const {
        items: aggregates,
        add: addAggregate,
        update: updateAggregate,
        remove: removeAggregate,
        normalized: normalizedAggregates,
        incomplete: incompleteAggregate
    } = useComputedColumnList()
    const conditions = ref<DraftNode[]>([])
    const conditionsBool = ref<'AND' | 'OR'>('AND')

    const columnType = (name: string) => columns.value.find((c) => c.name === name)?.type ?? ''
    const coerce = (column: string, value: string) => coerceByType(columnType(column), value)

    const spec = computed<Omit<AggregateSpec, 'newTableName'>>(() => ({
        groupBy: groupBy.value,
        aggregates: normalizedAggregates.value,
        conditions: pruneActive(conditions.value, coerce)
    }))

    // The one shape the server refuses outright ("an aggregation needs at
    // least one grouped column or aggregate") — every other combination
    // (grouped alone, aggregated alone, both) is a valid GROUP BY.
    const isEmpty = computed(
        () => (spec.value.groupBy?.length ?? 0) === 0 && (spec.value.aggregates?.length ?? 0) === 0
    )

    function toggleGroupBy(name: string) {
        groupBy.value = toggleInList(groupBy.value, name)
    }

    function submit(newTableName: string) {
        return run(
            () => aggregateTableFromTable(tableId.value, { newTableName, ...spec.value }),
            "Impossible d'agréger la table"
        )
    }

    return {
        isLoading,
        error,
        groupBy,
        aggregates,
        conditions,
        conditionsBool,
        incompleteAggregate,
        isEmpty,
        toggleGroupBy,
        addAggregate,
        updateAggregate,
        removeAggregate,
        submit
    }
}
