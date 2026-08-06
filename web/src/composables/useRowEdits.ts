import { computed, reactive, ref, type Ref } from 'vue'
import type { ColumnSchema, Row, RowValue } from '../client/types'
import { coerceByType, columnType, typeError } from '../utils/columnTypes'
import { cellValue } from '../utils/rows'

export type RowUpdate = { pkey: string; values: Record<string, unknown> }

// The key a row that does not exist yet is held under. A draft has no primary
// key — the server mints it on insert — but the grid, the draft store and the
// row's own fields all need to name it, and giving it a key of its own is
// what lets a new row be nothing but a row whose cells are all empty.
const DRAFT_PREFIX = '__draft:'

export function isDraft(pkey: string): boolean {
    return pkey.startsWith(DRAFT_PREFIX)
}

// What an edit session has written into the grid: the cells it retyped on
// existing rows, and the rows it is about to create. Both are held until the
// session's *Enregistrer* — the same contract the geometries follow: a
// keystroke is not a submission, what is pending is marked, and *Annuler*
// drops the lot. Which is why the grid has no per-row edit button any more:
// the mode is what makes a cell writable, and a row is nothing but its cells.
//
// A draft row is created empty, on purpose. On a geo table it is created by
// the *drawing* — one shape drawn, one row — so there is nothing to fill in
// before the shape exists, and the attributes are typed in the grid like any
// others rather than in a modal that would ask for them at the one moment the
// user is looking at the map (§8.3).
//
// Nothing here talks to the network. A session writes everything it holds
// through useLayerEditing.save, which is also what pairs the drafts with the
// geometries drawn for them — in order, the nth shape being the nth row.
export function useRowEdits(columns: Ref<ColumnSchema[]>, editableColumns: Ref<string[]>) {
    const active = ref(false)
    // pkey → column → the text in the field. A cell brought back to its stored
    // value leaves the draft entirely: the count has to state what a save would
    // actually write, not what was clicked into.
    const drafts = reactive<Record<string, Record<string, string>>>({})
    // The rows to create, in creation order — the order the geometries drawn
    // for them come back in. Keys, not indices: a draft dropped from the middle
    // must not silently hand its typed values to the one that followed it.
    const draftKeys = ref<string[]>([])
    let nextKey = 0

    // What the grid shows for a scalar, and what a field starts from — the same
    // rendering, so entering the mode never rewrites a cell under the user.
    function asText(value: RowValue | undefined): string {
        return value === null || value === undefined ? '' : String(value)
    }

    function stored(row: Row, column: string): string {
        return isDraft(row.pkey) ? '' : asText(cellValue(row, column))
    }

    function display(row: Row, column: string): string {
        return drafts[row.pkey]?.[column] ?? stored(row, column)
    }

    function isDirty(pkey: string, column: string): boolean {
        return drafts[pkey]?.[column] !== undefined
    }

    function setCell(row: Row, column: string, value: string) {
        const draft = drafts[row.pkey] ?? (drafts[row.pkey] = {})
        if (value === stored(row, column)) delete draft[column]
        else draft[column] = value
        if (Object.keys(draft).length === 0) delete drafts[row.pkey]
    }

    // The rows to create, as the grid shows them: at the top, empty, and
    // writable like any other. They carry no value of their own — what has been
    // typed into them lives in the same draft store as every other cell.
    const draftRows = computed<Row[]>(() => draftKeys.value.map((pkey) => ({ pkey })))

    function addDraft(): string {
        const pkey = `${DRAFT_PREFIX}${nextKey++}`
        draftKeys.value.push(pkey)
        return pkey
    }

    function removeDraft(pkey: string) {
        draftKeys.value = draftKeys.value.filter((key) => key !== pkey)
        delete drafts[pkey]
    }

    // On a geo table the drawing is what creates a row: the session holds as
    // many drafts as it has shapes, and one appears the moment a shape is
    // closed. Shapes are only ever added within a session (a drawn shape is
    // undone by dropping the session, LIM-F5), so this only ever grows — but it
    // is written to hold either way, since it is the map that counts.
    function syncDrafts(count: number) {
        while (draftKeys.value.length < count) addDraft()
        while (draftKeys.value.length > count) {
            const pkey = draftKeys.value[draftKeys.value.length - 1]
            removeDraft(pkey)
        }
    }

    // What a save would refuse: cells whose text does not fit their column's
    // type. The check is the grid's, not the server's — a write refused for the
    // whole session names neither the row nor the column at fault (§8.2).
    const invalidCount = computed(
        () =>
            Object.values(drafts)
                .flatMap((values) => Object.entries(values))
                .filter(
                    ([column, text]) => typeError(columnType(columns.value, column), text) !== null
                ).length
    )

    // Rows retyped and rows created, counted apart: they are two different
    // writes and they fail differently (§8.2).
    const dirtyCount = computed(() => Object.keys(drafts).filter((pkey) => !isDraft(pkey)).length)
    const draftCount = computed(() => draftKeys.value.length)

    function coerce(column: string, text: string): unknown {
        return coerceByType(columnType(columns.value, column), text)
    }

    // Only the columns actually retyped are sent. A save that resent the whole
    // row would write back, as the text a grid displayed, columns the user
    // never touched — and every column the grid does not even show.
    const pendingUpdates = computed<RowUpdate[]>(() =>
        Object.entries(drafts)
            .filter(([pkey]) => !isDraft(pkey))
            .map(([pkey, values]) => ({
                pkey,
                values: Object.fromEntries(
                    Object.entries(values).map(([column, text]) => [column, coerce(column, text)])
                )
            }))
    )

    // An insert states every editable column, empty ones included: a row is
    // created whole, and a column left out of the request is not the same thing
    // as a column created empty.
    const pendingInserts = computed<Record<string, unknown>[]>(() =>
        draftKeys.value.map((pkey) =>
            Object.fromEntries(
                editableColumns.value.map((column) => [
                    column,
                    coerce(column, drafts[pkey]?.[column] ?? '')
                ])
            )
        )
    )

    function reset() {
        for (const pkey of Object.keys(drafts)) delete drafts[pkey]
        draftKeys.value = []
    }

    function start() {
        reset()
        active.value = true
    }

    function stop() {
        reset()
        active.value = false
    }

    return {
        active,
        display,
        isDirty,
        setCell,
        draftRows,
        addDraft,
        removeDraft,
        syncDrafts,
        dirtyCount,
        draftCount,
        invalidCount,
        pendingUpdates,
        pendingInserts,
        reset,
        start,
        stop
    }
}
