import { computed, ref } from 'vue'
import type { ComputedColumn } from '../client/types'

// The alias/expression list machinery `useDeriveTable`'s computed columns and
// `useAggregateFromTable`'s aggregates share: add/update/remove, the
// trim-and-drop-empty normalisation a submitted spec applies, and whether any
// entry is half filled in — blocked here too, with the difference that here
// it can be pointed at rather than only refused by the server.
export function useComputedColumnList() {
    const items = ref<ComputedColumn[]>([])

    function add() {
        items.value = [...items.value, { alias: '', expression: '' }]
    }

    function update(index: number, patch: Partial<ComputedColumn>) {
        items.value = items.value.map((c, i) => (i === index ? { ...c, ...patch } : c))
    }

    function remove(index: number) {
        items.value = items.value.filter((_, i) => i !== index)
    }

    // What a submitted spec actually sends: trimmed, entries left wholly
    // empty dropped rather than sent as a blank alias/expression pair.
    const normalized = computed(() =>
        items.value
            .filter((c) => c.alias.trim() !== '' || c.expression.trim() !== '')
            .map((c) => ({ alias: c.alias.trim(), expression: c.expression.trim() }))
    )

    const incomplete = computed(() =>
        normalized.value.some((c) => c.alias === '' || c.expression === '')
    )

    return { items, add, update, remove, normalized, incomplete }
}
