import { ref, type Ref } from 'vue'
import { getRowGeometry } from '../client/tables'
import { errorMessage } from '../client/http'
import type { EditableFeature } from './useEditLayer'
import { useToast } from './useToast'

// How many rows a session may hold at once, counted over the **session**:
// editing a boundary means panning along it, and a cap counted per view would
// let a dozen pans pile up a dozen capfuls. It is not a budget the ordinary
// gesture ever meets — a session grows one geometry per geometry pointed at —
// but a long afternoon of hovering must not end with a `Modify` and a `Snap`
// indexing more vertices than they can keep interactive (§8.6).
export const MAX_EDITABLE = 500

// What an edit session has on the map: the geometries the pointer has
// reached, at full resolution — never a tile's simplified copy (§8.8).
//
// Nothing is loaded on entering the mode. Loading everything the view held
// meant tens of megabytes of GeoJSON, thousands of OpenLayers features and a
// `Snap` index over every one of their vertices, all built before the user
// had said what they meant to touch — which is what froze the tab on entering
// the mode and again on leaving it. Here entering costs nothing and a hover
// costs one geometry.
//
// It only ever hands back what the layer does not already hold: the map keeps
// geometries the user may have moved, and the server's copy of one of them
// must never come back over it.
export function useEditFeatures(tableId: Ref<string>, active: Ref<boolean>) {
    const toast = useToast()
    const loading = ref(false)
    // The geometry the last reach brought in — never the whole set, since
    // handing back a row the map already holds would overwrite one the user
    // moved (useEditLayer.load).
    const features = ref<EditableFeature[]>([])
    // Every pkey the session has asked for, answered or not. The pointer rests
    // over the same row for many frames and the caller cannot tell them apart,
    // so this is where a hover becomes one request.
    const requested = new Set<string>()
    // How many requests are in flight, not whether one is: several hovers in a
    // row overlap, and the first to come back must not clear the indicator the
    // others are still owed.
    let pending = 0

    function reset() {
        requested.clear()
        features.value = []
        pending = 0
        loading.value = false
    }

    // The row the pointer rests over, fetched whole. Best effort: a geometry
    // that fails to load leaves the session with what it already holds rather
    // than closing it under the user's hands.
    async function reach(pkey: string) {
        if (!active.value || requested.has(pkey) || requested.size >= MAX_EDITABLE) return
        requested.add(pkey)
        pending += 1
        loading.value = true
        try {
            const { geometry } = await getRowGeometry(tableId.value, pkey)
            // The session may have been left while this was in flight; loading its
            // answer would put a feature on a layer that is no longer there.
            if (!active.value || !geometry) return
            features.value = [{ pkey, geometry }]
        } catch (e) {
            // Asked again on the next hover rather than left unreachable for the
            // rest of the session.
            requested.delete(pkey)
            toast.error(errorMessage(e, 'Impossible de charger la géométrie à modifier'))
        } finally {
            pending -= 1
            if (pending <= 0) {
                pending = 0
                loading.value = false
            }
        }
    }

    // What the map has stopped holding — untouched rows the view has left
    // (useEditLayer.prune). Their room comes back, which is what makes the cap
    // a working set rather than a one-way ratchet, and it is also what lets a
    // row be reached again after the session let go of it.
    function release(pkeys: string[]) {
        for (const pkey of pkeys) requested.delete(pkey)
    }

    return { features, loading, reach, release, reset }
}
