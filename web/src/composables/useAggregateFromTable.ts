import { computed, ref, type Ref } from 'vue'
import { aggregateTableFromTable } from '../client/tables'
import { errorMessage } from '../client/http'
import type { AggregateColumn, AggregateSpec, ColumnSchema } from '../client/types'
import { coerceByType } from '../utils/columnTypes'
import { pruneActive, type DraftNode } from '../utils/filterTree'

// The guided aggregation from the table being viewed: GROUP BY columns plus
// aggregate expressions — the aggregation counterpart of useDeriveTable's
// guided mode, but its own composable rather than a third mode there
// (TODO-F37). The two share no state: a column is either carried over
// (row-wise) or grouped/aggregated (here), never both, and a shared
// column-selection concept would let a column be "selected" without being
// either — which GROUP BY refuses as a query error, not a default.
export function useAggregateFromTable(tableId: Ref<string>, columns: Ref<ColumnSchema[]>) {
    const isLoading = ref(false)
    const error = ref('')

    const groupBy = ref<string[]>([])
    const aggregates = ref<AggregateColumn[]>([])
    const conditions = ref<DraftNode[]>([])
    const conditionsBool = ref<'AND' | 'OR'>('AND')

    const columnType = (name: string) => columns.value.find((c) => c.name === name)?.type ?? ''
    const coerce = (column: string, value: string) => coerceByType(columnType(column), value)

    const spec = computed<Omit<AggregateSpec, 'newTableName'>>(() => ({
        groupBy: groupBy.value,
        aggregates: aggregates.value
            .filter((a) => a.alias.trim() !== '' || a.expression.trim() !== '')
            .map((a) => ({ alias: a.alias.trim(), expression: a.expression.trim() })),
        conditions: pruneActive(conditions.value, coerce)
    }))

    // An aggregate column half filled in is rejected by the server, so it is
    // blocked here too — with the difference that here it can be pointed at.
    const incompleteAggregate = computed(() =>
        (spec.value.aggregates ?? []).some((a) => a.alias === '' || a.expression === '')
    )

    // The one shape the server refuses outright ("an aggregation needs at
    // least one grouped column or aggregate") — every other combination
    // (grouped alone, aggregated alone, both) is a valid GROUP BY.
    const isEmpty = computed(
        () => (spec.value.groupBy?.length ?? 0) === 0 && (spec.value.aggregates?.length ?? 0) === 0
    )

    function toggleGroupBy(name: string) {
        groupBy.value = groupBy.value.includes(name)
            ? groupBy.value.filter((c) => c !== name)
            : [...groupBy.value, name]
    }

    function addAggregate() {
        aggregates.value = [...aggregates.value, { alias: '', expression: '' }]
    }

    function updateAggregate(index: number, patch: Partial<AggregateColumn>) {
        aggregates.value = aggregates.value.map((a, i) => (i === index ? { ...a, ...patch } : a))
    }

    function removeAggregate(index: number) {
        aggregates.value = aggregates.value.filter((_, i) => i !== index)
    }

    async function submit(newTableName: string) {
        isLoading.value = true
        error.value = ''
        try {
            return await aggregateTableFromTable(tableId.value, { newTableName, ...spec.value })
        } catch (e) {
            error.value = errorMessage(e, "Impossible d'agréger la table")
            return null
        } finally {
            isLoading.value = false
        }
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
