import { computed, ref, watch, type Ref } from 'vue'
import type { Row } from '../client/types'

// How the grid arranges what is selected, and nothing else — the selection
// itself is useFeatureSelection's.
//
// 'inline' is the default and the only honest one: a selected row stays where
// the sort put it. Lifting it the moment it is clicked moves the grid under
// the pointer, and the next click lands on a different row. The two other
// arrangements are asked for explicitly, from the selection bar.
export type SelectionView = 'inline' | 'pinned' | 'only'

export function useSelectionView(
    pageRows: Ref<Row[]>,
    draftRows: Ref<Row[]>,
    // Hydrated rather than read off the page: a selection made on the map holds
    // rows the grid has never loaded, and they only ever appear here.
    selectedRows: Ref<Row[]>
) {
    const view = ref<SelectionView>('inline')

    // Both controls are toggles onto the same state: asking for one arrangement
    // leaves the other, and asking again returns to 'inline'.
    function toggle(mode: 'pinned' | 'only') {
        view.value = view.value === mode ? 'inline' : mode
    }

    // An arrangement of the selection means nothing without one: an emptied
    // selection that left 'only' behind would show an empty grid over a full
    // table.
    watch(selectedRows, (rows) => {
        if (rows.length === 0) view.value = 'inline'
    })

    // The session's drafts always come first: they are empty, and a row with no
    // value to sort on has no place among the sorted ones.
    const displayRows = computed<Row[]>(() => {
        const drafts = draftRows.value
        if (view.value === 'only') return [...drafts, ...selectedRows.value]
        if (view.value === 'inline') return [...drafts, ...pageRows.value]
        const selected = new Set(selectedRows.value.map((row) => row.pkey))
        return [
            ...drafts,
            ...selectedRows.value,
            ...pageRows.value.filter((row) => !selected.has(row.pkey))
        ]
    })

    return { view, toggle, displayRows }
}
