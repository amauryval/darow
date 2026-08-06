import { ref, type Ref } from 'vue'
import { getDistinctValues } from '../client/tables'
import type { RowValue } from '../client/types'
import { errorMessage } from '../client/http'

// The full distinct-value listing behind the columns modal's "Voir les
// valeurs" toggle — one instance for the whole modal, one column expanded at
// a time, and every column's list kept once fetched: closing and reopening
// the same column costs nothing.
export function useColumnDistinctValues(tableId: Ref<string>) {
    const expandedColumn = ref<string | null>(null)
    const values = ref<RowValue[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)
    const cache = new Map<string, RowValue[]>()

    async function toggle(column: string) {
        if (expandedColumn.value === column) {
            expandedColumn.value = null
            return
        }
        expandedColumn.value = column
        error.value = null
        const cached = cache.get(column)
        if (cached) {
            values.value = cached
            return
        }
        loading.value = true
        try {
            const res = await getDistinctValues(tableId.value, column)
            cache.set(column, res.values)
            values.value = res.values
        } catch (e) {
            error.value = errorMessage(e, `Impossible de lister les valeurs de ${column}`)
            expandedColumn.value = null
        } finally {
            loading.value = false
        }
    }

    return { expandedColumn, values, loading, error, toggle }
}
