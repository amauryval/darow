import { onScopeDispose, ref, type Ref } from 'vue'
import { getDistinctValues } from '../client/tables'
import type { RowValue } from '../client/types'

// The server pages distinct values, so a suggestion list is always the first
// SUGGESTION_LIMIT matches of what is typed — never the column's whole
// cardinality.
const SUGGESTION_LIMIT = 20
// Typing "Par" would otherwise fire three queries; only the pause matters.
const DEBOUNCE_MS = 200

// Suggests the values a column actually holds, narrowed by what the user is
// typing. One instance per input: two filter rows suggest on two different
// columns and must not share a list.
export function useColumnValues(tableId: Ref<string>, column: Ref<string>) {
    const values = ref<RowValue[]>([])
    const loading = ref(false)
    // A slower request must never overwrite a newer one's result: only the
    // latest sequence number is allowed to publish.
    let sequence = 0
    let timer: ReturnType<typeof setTimeout> | undefined

    function cancelPending() {
        if (timer !== undefined) clearTimeout(timer)
        timer = undefined
    }

    async function fetchValues(search: string, requested: number) {
        loading.value = true
        try {
            const res = await getDistinctValues(tableId.value, column.value, {
                search,
                limit: SUGGESTION_LIMIT
            })
            if (requested !== sequence) return
            values.value = res.values
        } catch {
            // A suggestion list is an assistance, not the user's action: a column
            // the server refuses to enumerate leaves the input plainly typable
            // rather than raising an error over a filter being written (§8.4).
            if (requested === sequence) values.value = []
        } finally {
            if (requested === sequence) loading.value = false
        }
    }

    function search(text: string) {
        cancelPending()
        const requested = ++sequence
        timer = setTimeout(() => fetchValues(text, requested), DEBOUNCE_MS)
    }

    function reset() {
        cancelPending()
        sequence++
        values.value = []
        loading.value = false
    }

    onScopeDispose(cancelPending)

    return { values, loading, search, reset }
}
