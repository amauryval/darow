import { describe, expect, it } from 'vitest'
import { nextTick, ref } from 'vue'
import { useSelectionView } from './useSelectionView'
import type { Row } from '../client/types'

function row(pkey: string): Row {
    return { pkey, name: pkey }
}

function setup() {
    const pageRows = ref<Row[]>([row('a'), row('b'), row('c')])
    const draftRows = ref<Row[]>([])
    const selectedRows = ref<Row[]>([row('b')])
    return {
        pageRows,
        draftRows,
        selectedRows,
        ...useSelectionView(pageRows, draftRows, selectedRows)
    }
}

describe('useSelectionView', () => {
    it('leaves a selected row where the sort put it by default', () => {
        const { view, displayRows } = setup()

        expect(view.value).toBe('inline')
        expect(displayRows.value.map((r) => r.pkey)).toEqual(['a', 'b', 'c'])
    })

    it('lifts the selection above the page when pinned', () => {
        const { toggle, displayRows } = setup()

        toggle('pinned')

        expect(displayRows.value.map((r) => r.pkey)).toEqual(['b', 'a', 'c'])
    })

    it('shows the selection alone, including rows the grid never loaded', () => {
        const { selectedRows, toggle, displayRows } = setup()
        selectedRows.value = [row('z')]

        toggle('only')

        expect(displayRows.value.map((r) => r.pkey)).toEqual(['z'])
    })

    it('returns to inline when the same arrangement is asked for again', () => {
        const { view, toggle } = setup()

        toggle('pinned')
        toggle('pinned')

        expect(view.value).toBe('inline')
    })

    it('leaves one arrangement when the other is asked for', () => {
        const { view, toggle } = setup()

        toggle('pinned')
        toggle('only')

        expect(view.value).toBe('only')
    })

    it('falls back to inline once the selection is emptied', async () => {
        const { selectedRows, view, toggle } = setup()
        toggle('only')

        selectedRows.value = []
        await nextTick()

        expect(view.value).toBe('inline')
    })

    it('puts the session’s drafts first, whatever the arrangement', () => {
        const { draftRows, toggle, displayRows } = setup()
        draftRows.value = [row('draft')]

        expect(displayRows.value[0].pkey).toBe('draft')

        toggle('pinned')
        expect(displayRows.value[0].pkey).toBe('draft')

        toggle('only')
        expect(displayRows.value.map((r) => r.pkey)).toEqual(['draft', 'b'])
    })
})
