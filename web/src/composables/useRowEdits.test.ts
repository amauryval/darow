import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { isDraft, useRowEdits } from './useRowEdits'
import type { ColumnSchema, Row } from '../client/types'

const columns: ColumnSchema[] = [
    { name: 'name', type: 'VARCHAR' },
    { name: 'height', type: 'INTEGER' }
]

const row: Row = { pkey: 'p1', name: 'Nantes', height: 10 }

function setup() {
    return useRowEdits(ref(columns), ref(['name', 'height']))
}

describe('isDraft', () => {
    it('tells a row that does not exist yet from a stored one', () => {
        expect(isDraft('__draft:0')).toBe(true)
        expect(isDraft('p1')).toBe(false)
    })
})

describe('useRowEdits', () => {
    it('starts inactive, holding nothing', () => {
        const { active, dirtyCount, draftCount, invalidCount } = setup()

        expect(active.value).toBe(false)
        expect(dirtyCount.value).toBe(0)
        expect(draftCount.value).toBe(0)
        expect(invalidCount.value).toBe(0)
    })

    it('shows the stored value until a cell is retyped', () => {
        const { display, isDirty, setCell } = setup()

        expect(display(row, 'name')).toBe('Nantes')
        expect(isDirty('p1', 'name')).toBe(false)

        setCell(row, 'name', 'Rennes')

        expect(display(row, 'name')).toBe('Rennes')
        expect(isDirty('p1', 'name')).toBe(true)
    })

    it('renders a null cell as an empty field', () => {
        const { display } = setup()

        expect(display({ pkey: 'p1', name: null }, 'name')).toBe('')
    })

    it('drops a cell brought back to its stored value from what a save would write', () => {
        const { setCell, isDirty, dirtyCount } = setup()
        setCell(row, 'name', 'Rennes')

        setCell(row, 'name', 'Nantes')

        expect(isDirty('p1', 'name')).toBe(false)
        expect(dirtyCount.value).toBe(0)
    })

    it('counts a retyped row once, whatever the number of cells', () => {
        const { setCell, dirtyCount } = setup()

        setCell(row, 'name', 'Rennes')
        setCell(row, 'height', '20')

        expect(dirtyCount.value).toBe(1)
    })

    it('sends only the columns actually retyped, typed by their column', () => {
        const { setCell, pendingUpdates } = setup()

        setCell(row, 'height', '20')

        expect(pendingUpdates.value).toEqual([{ pkey: 'p1', values: { height: 20 } }])
    })

    it('creates a draft row empty, at the top, writable like any other', () => {
        const { addDraft, draftRows, draftCount, display } = setup()

        const pkey = addDraft()

        expect(draftRows.value).toEqual([{ pkey }])
        expect(draftCount.value).toBe(1)
        expect(display({ pkey }, 'name')).toBe('')
    })

    it('holds a draft’s typed values in the same store as any other cell', () => {
        const { addDraft, setCell, display } = setup()
        const pkey = addDraft()

        setCell({ pkey }, 'name', 'Nouvelle')

        expect(display({ pkey }, 'name')).toBe('Nouvelle')
    })

    it('drops a removed draft’s values rather than handing them to the next', () => {
        const { addDraft, setCell, removeDraft, draftRows, display } = setup()
        const first = addDraft()
        const second = addDraft()
        setCell({ pkey: first }, 'name', 'première')

        removeDraft(first)

        expect(draftRows.value).toEqual([{ pkey: second }])
        expect(display({ pkey: second }, 'name')).toBe('')
    })

    it('holds as many drafts as the map has shapes', () => {
        const { syncDrafts, draftCount } = setup()

        syncDrafts(3)
        expect(draftCount.value).toBe(3)

        syncDrafts(1)
        expect(draftCount.value).toBe(1)
    })

    it('states every editable column on an insert, empty ones included', () => {
        const { addDraft, setCell, pendingInserts } = setup()
        const pkey = addDraft()
        setCell({ pkey }, 'name', 'Nouvelle')

        expect(pendingInserts.value).toEqual([{ name: 'Nouvelle', height: null }])
    })

    it('counts the cells a save would be refused on, naming neither the server nor a guess', () => {
        const { setCell, invalidCount } = setup()

        setCell(row, 'height', 'dix')
        expect(invalidCount.value).toBe(1)

        setCell(row, 'height', '10')
        expect(invalidCount.value).toBe(0)
    })

    it('drops everything on reset, and on leaving the session', () => {
        const { active, addDraft, setCell, start, stop, dirtyCount, draftCount } = setup()
        start()
        addDraft()
        setCell(row, 'name', 'Rennes')

        stop()

        expect(active.value).toBe(false)
        expect(dirtyCount.value).toBe(0)
        expect(draftCount.value).toBe(0)
    })

    it('starts a session on a clean slate', () => {
        const { addDraft, start, active, draftCount } = setup()
        addDraft()

        start()

        expect(active.value).toBe(true)
        expect(draftCount.value).toBe(0)
    })
})
