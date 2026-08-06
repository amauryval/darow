import { ref, watch, type Ref } from 'vue'
import { addRows, updateRows } from '../client/tables'
import { errorMessage } from '../client/http'
import type { LayerChanges } from './useEditLayer'
import type { RowUpdate } from './useRowEdits'
import { useEditFeatures } from './useEditFeatures'
import { summarizeWrites } from '../utils/editPrompts'
import { useToast } from './useToast'

// The edit session of a table: what the mode is waiting for, and what leaving
// it writes. What it carries on the map is useEditFeatures'; the cells it has
// retyped are useRowEdits' — this is where the two are written, together.
//
// The write is server-authoritative: nothing is drawn as saved until the
// server answers, and every moved geometry goes in one request so a limit
// two rows share is never left moved on one side only (§8.2).
export function useLayerEditing(
    tableId: Ref<string>,
    geometryColumn: Ref<string | undefined>,
    onMutated: () => void
) {
    const toast = useToast()
    const active = ref(false)
    const saving = ref(false)
    const loaded = useEditFeatures(tableId, active)

    function reset() {
        active.value = false
        loaded.reset()
    }

    function start() {
        reset()
        active.value = true
    }

    // What the session writes, both halves at once: the geometries it reshaped
    // and the cells it retyped (useRowEdits). They land in a single request
    // when they touch the same row — a row whose limit moved *and* whose name
    // changed is one write, not two, and a save that half-failed would leave
    // the grid disagreeing with the map. Returns whether the session ended.
    async function save(
        changes: LayerChanges,
        rowUpdates: RowUpdate[] = [],
        rowInserts: Record<string, unknown>[] = []
    ): Promise<boolean> {
        const column = geometryColumn.value
        const { updates, inserts } = changes
        if (!column && (updates.size > 0 || inserts.length > 0)) {
            toast.error(
                "Cette table n'a pas de colonne géométrique : rien ne peut être enregistré."
            )
            return false
        }
        if (updates.size === 0 && rowUpdates.length === 0 && rowInserts.length === 0) {
            reset()
            return true
        }

        // One entry per row, whichever half touched it — the two are merged here
        // rather than sent one after the other, so a row is never written twice
        // in the same save.
        const merged = new Map<string, Record<string, unknown>>()
        for (const { pkey, values } of rowUpdates) merged.set(pkey, { ...values })
        if (column) {
            for (const [pkey, geometry] of updates)
                merged.set(pkey, { ...merged.get(pkey), [column]: geometry })
        }

        saving.value = true
        try {
            // Every touched row in one request: the server writes them in a single
            // transaction, so a shared limit is never half-written.
            if (merged.size > 0) {
                await updateRows(
                    tableId.value,
                    [...merged].map(([pkey, values]) => ({ pkey, values }))
                )
            }
            // One drawn shape is one row, in order: the nth shape is the geometry
            // of the nth draft row of the grid, which is the row the drawing itself
            // created (§8.3). A table without geometry has drafts and no shapes.
            if (rowInserts.length > 0) {
                await addRows(
                    tableId.value,
                    rowInserts.map((values, index) =>
                        column && inserts[index] !== undefined
                            ? { ...values, [column]: inserts[index] }
                            : values
                    )
                )
            }
            toast.success(summarizeWrites(updates.size, rowInserts.length, rowUpdates.length))
            reset()
            onMutated()
            return true
        } catch (e) {
            toast.error(errorMessage(e, "Impossible d'enregistrer les modifications"))
            return false
        } finally {
            saving.value = false
        }
    }

    function cancel() {
        reset()
    }

    // A session belongs to the table it was opened on: its pkeys and its draft
    // mean nothing in another one (§9).
    watch(tableId, reset)

    return {
        active,
        saving,
        loading: loaded.loading,
        features: loaded.features,
        reach: loaded.reach,
        release: loaded.release,
        start,
        save,
        cancel
    }
}
