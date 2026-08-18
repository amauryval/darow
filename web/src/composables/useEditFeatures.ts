import { ref, type Ref } from 'vue'
import { getRowGeometry } from '../client/tables'
import { getNeighbours } from '../client/geo'
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

// How far around a reached geometry its neighbours are pulled in, in meters.
// Editing is topological: a limit two rows share only moves on both sides if
// both are in the session, and a vertex only snaps onto a neighbour that is
// there — so pointing at a geometry has to bring in what it touches, or the
// two tools answer nothing until every side has been hovered in turn (§8.8).
//
// A few meters rather than zero: a coverage digitized by hand leaves slivers,
// and a limit a meter apart is one the user still means to snap onto.
const NEIGHBOUR_BUFFER = 10

// How many neighbours one reach may bring in. A hover is cheap and must stay
// cheap: what surrounds a parcel is a handful of rows, and a geometry sitting
// on a dense layer must not pull a hundred of them in behind the pointer.
const MAX_NEIGHBOURS = 20

// What an edit session has on the map: the geometries the pointer has
// reached and what surrounds them, at full resolution — never a tile's
// simplified copy (§8.8).
//
// Nothing is loaded on entering the mode. Loading everything the view held
// meant tens of megabytes of GeoJSON, thousands of OpenLayers features and a
// `Snap` index over every one of their vertices, all built before the user
// had said what they meant to touch — which is what froze the tab on entering
// the mode and again on leaving it. Here entering costs nothing and a hover
// costs one geometry and the handful that touch it.
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

    // The rows around one geometry, pulled in with it so topology and snapping
    // have both sides of a shared limit. Best effort and silent: the row the
    // pointer reached is what the user asked for, and losing its surroundings
    // is not worth a toast over the map.
    async function reachNeighbours(geometry: GeoJSON.Geometry) {
        const room = MAX_EDITABLE - requested.size
        if (room <= 0) return
        try {
            const { features: found } = await getNeighbours(
                tableId.value,
                geometry,
                NEIGHBOUR_BUFFER,
                [...requested],
                Math.min(MAX_NEIGHBOURS, room)
            )
            if (!active.value) return
            const around = found.flatMap((f) =>
                f.geometry && !requested.has(f.id) ? [{ pkey: f.id, geometry: f.geometry }] : []
            )
            if (around.length === 0) return
            for (const f of around) requested.add(f.pkey)
            features.value = around
        } catch {
            // Nothing to say: the reached row is on the map, and the next hover
            // asks again.
        }
    }

    // The row the pointer rests over, fetched whole, and what surrounds it.
    // Best effort: a geometry that fails to load leaves the session with what
    // it already holds rather than closing it under the user's hands.
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
            await reachNeighbours(geometry)
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
