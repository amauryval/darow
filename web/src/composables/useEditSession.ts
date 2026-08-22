import { computed, ref, type Ref } from 'vue'
import type { LayerChanges } from './useEditLayer'
import type { useLayerEditing } from './useLayerEditing'
import type { useRowEdits } from './useRowEdits'
import { useToast } from './useToast'

const NO_GEOMETRY_CHANGES: LayerChanges = { updates: new Map(), inserts: [] }

// The table's edit session: one mode covering both halves of the object, the
// rows and the geometries, never two (§8.3). Owns the session's state and the
// quartet that opens, cancels, discards and saves it, wherever the panel
// wires them to — the toolbar's toggle, the bar's controls, the map's own
// save/cancel.
export function useEditSession(
    rowEdits: ReturnType<typeof useRowEdits>,
    layerEditing: ReturnType<typeof useLayerEditing>,
    isGeo: Ref<boolean>
) {
    const toast = useToast()

    const active = computed(() => rowEdits.active.value || layerEditing.active.value)

    // The geometry half of what the session holds. The grid counts its own
    // retyped cells and drafts; a reshape only exists on the layer, so the map
    // reports it (MapView's `changed-count`).
    const changedGeometries = ref(0)

    // Everything one *Annuler* would drop, both halves at once — what the exit
    // has to be confirmed against (§8.3).
    const unsavedCount = computed(
        () => rowEdits.dirtyCount.value + rowEdits.draftCount.value + changedGeometries.value
    )
    const confirmingCancel = ref(false)

    function toggle() {
        if (active.value) {
            cancel()
            return
        }
        rowEdits.start()
        if (isGeo.value) layerEditing.start()
    }

    // Every way out of the mode goes through here — the *Éditer* toggle, the
    // bar's *Annuler*, the map's — so the session's work is protected once
    // rather than at each of them. An empty session is left with no question
    // to answer: the confirmation is a barrier in front of losing work, and
    // there is nothing to lose (§8.3).
    function cancel() {
        if (unsavedCount.value > 0) {
            confirmingCancel.value = true
            return
        }
        discard()
    }

    // Drops both halves at once — the reshapes and the retyped cells are one
    // session. It says what it drops: a session opened to give a new row its
    // geometry loses that row (useLayerEditing.cancel).
    function discard() {
        confirmingCancel.value = false
        changedGeometries.value = 0
        layerEditing.cancel()
        rowEdits.stop()
    }

    // One *Enregistrer* for the whole session, wherever its button stands: the
    // geometries the map committed and the cells the grid holds are written
    // together, and the mode only closes if the write landed.
    async function save(changes: LayerChanges = NO_GEOMETRY_CHANGES) {
        // A cell whose text does not fit its column is refused here, where the
        // grid can point at it, rather than by the server for the whole
        // session at once.
        const invalid = rowEdits.invalidCount.value
        if (invalid > 0) {
            toast.error(
                `${invalid} valeur(s) ne correspondent pas au type de leur colonne : corrigez-les avant d'enregistrer.`
            )
            return
        }
        if (
            await layerEditing.save(
                changes,
                rowEdits.pendingUpdates.value,
                rowEdits.pendingInserts.value
            )
        ) {
            changedGeometries.value = 0
            rowEdits.stop()
        }
    }

    return {
        active,
        unsavedCount,
        confirmingCancel,
        changedGeometries,
        toggle,
        cancel,
        discard,
        save
    }
}
