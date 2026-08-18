import { computed, ref, type Ref } from 'vue'
import type OlMap from 'ol/Map'
import Collection from 'ol/Collection'
import Feature from 'ol/Feature'
import VectorLayer from 'ol/layer/Vector'
import VectorSource from 'ol/source/Vector'
import type Geometry from 'ol/geom/Geometry'
import type { Pixel } from 'ol/pixel'
import MultiPoint from 'ol/geom/MultiPoint'
import type { Coordinate } from 'ol/coordinate'
import {
    DIRTY_PROPERTY,
    MOVED_PROPERTY,
    PKEY_PROPERTY,
    coordinateKey,
    editStyle,
    focusStyle,
    verticesOf
} from '../utils/drawStyles'
import { isAreal } from '../utils/holes'
import { useMoveLeaders } from './useMoveLeaders'
import { readGeometry, writeGeometry } from '../utils/projection'

// A row as the edit layer holds it: the row it is, and its geometry at full
// resolution — never a tile's simplified copy (§8.8).
export interface EditableFeature {
    pkey: string
    geometry: GeoJSON.Geometry
}

// What a session would write: the rows whose geometry moved, and the shapes
// drawn that belong to no row yet.
export interface LayerChanges {
    updates: Map<string, GeoJSON.Geometry>
    inserts: GeoJSON.Geometry[]
}

// The one source an edit session works on, holding every feature of the
// layer that is in view — not one row and its neighbours.
//
// That single source is what makes editing topological: two geometries that
// were snapped hold the same coordinate, and one Modify over one source
// drags every geometry carrying that coordinate at once. The limit moves on
// both sides in one gesture, with nothing recomputed afterwards and no way
// for the two sides to drift apart.
// The table's own rendering stays on under a session, so the editable copies
// must be drawn above it whatever order the layers were added in — a render
// mode toggled during the session would otherwise land on top and hide the
// very geometries being reshaped.
const EDIT_Z_INDEX = 10

// A vertex is grabbed from a few pixels away, so the feature carrying it has
// to be recognised from at least as far.
const HIT_TOLERANCE = 8

// How many **untouched** geometries a session keeps around. Not zero: a
// neighbour has to be in the source for a shared limit to move on both sides,
// and it has to be there for a new vertex to snap onto it — dropping every
// geometry the user did not modify would take topology and snapping with it.
// Past this, the ones reached longest ago are given back: they are weight in
// the `Snap`'s index and in the `Modify`'s, and the user has moved on. Held
// several hovers deep, since one hover brings in a geometry *and* what
// touches it (useEditFeatures) — a cap of one batch would evict the
// neighbours of the parcel being worked on. What was touched is never counted
// here and never released — it carries unsaved work.
const MAX_RESTING = 120

export function useEditLayer(map: Ref<OlMap | null>) {
    const source = new VectorSource()
    const layer = new VectorLayer({ source, style: editStyle, zIndex: EDIT_Z_INDEX })
    // The vertices live on a layer of their own, holding at most the one
    // feature the pointer rests over. Drawing them from the edit layer's own
    // style would mean rebuilding the whole view's dots on every frame of every
    // pan and every drag, and repainting every geometry in view each time the
    // pointer crossed a new one. Here a hover costs one geometry.
    //
    // It carries the very feature it decorates, not a copy: a drag moves that
    // geometry, and the dots have to follow it rather than mark where it was.
    const focusSource = new VectorSource()
    const focusLayer = new VectorLayer({
        source: focusSource,
        style: focusStyle,
        zIndex: EDIT_Z_INDEX + 1
    })
    // Above both: a leader is an annotation over the geometry it describes, and
    // it has to stay readable across the dots of the feature it starts from.
    const leaders = useMoveLeaders(EDIT_Z_INDEX + 2)
    // What the session has touched so far, split the way the write is: rows
    // whose geometry moved go out as an update, shapes that belong to no row
    // yet as an insert. The bar states both before the user commits to
    // sending them, because they do not fail the same way — a new shape with
    // no attributes is what a save refuses.
    const changedCount = ref(0)
    const createdCount = ref(0)
    const dirtyCount = computed(() => changedCount.value + createdCount.value)
    // Whether anything on the layer can hold a hole at all. The hole tool is
    // meaningless on a layer of lines or points, and a tool that answers
    // nothing is worse than one that says it does not apply (§8.2).
    const hasAreal = ref(false)
    let added = false
    // Filling the layer from the server is not a change the user made. Without
    // this, every pan would mark the rows it revealed as modified and offer to
    // write them back unchanged.
    let filling = false

    function add() {
        const m = map.value
        if (!m || added) return
        added = true
        m.addLayer(layer)
        m.addLayer(focusLayer)
        m.addLayer(leaders.layer)
    }

    // The single feature the pointer rests over, as a collection because that
    // is what a Modify restricted to one geometry follows. Kept before the
    // press: by the time a drag starts, the interaction has already chosen.
    const solo = new Collection<Feature<Geometry>>()
    // The same feature, held plainly: it is also the one whose vertices are
    // drawn, whether or not the Modify happens to be restricted to it.
    let focused: Feature<Geometry> | null = null

    function focus(next: Feature<Geometry> | null) {
        if (focused === next) return
        focused = next
        solo.clear()
        focusSource.clear()
        if (next) {
            solo.push(next)
            focusSource.addFeature(next)
        }
    }

    // Hit detection is not free, so the caller owns the throttling and hands a
    // pixel rather than every pointer event. Answers whether the session
    // already holds something under that pixel: what it does not hold is what
    // the caller goes and fetches (§8.8).
    function trackPointer(m: OlMap, pixel: Pixel): boolean {
        const found = m.forEachFeatureAtPixel(pixel, (feature) => feature, {
            layerFilter: (candidate) => candidate === layer,
            hitTolerance: HIT_TOLERANCE
        })
        const hit = found instanceof Feature ? found : null
        focus(hit)
        return hit !== null
    }

    function pkeyOf(feature: Feature<Geometry>): string | null {
        const pkey = feature.get(PKEY_PROPERTY)
        return typeof pkey === 'string' ? pkey : null
    }

    // Adds what the view has gained without touching what is already there: a
    // feature the user has moved must never be replaced by the server's copy
    // of it, which is what a plain reload would do on every pan.
    function load(features: EditableFeature[]) {
        const known = new Set(source.getFeatures().flatMap((f) => (pkeyOf(f) ? [pkeyOf(f)] : [])))
        const fresh = features.filter((f) => !known.has(f.pkey))
        filling = true
        const built = fresh.map((f) => {
            const feature = new Feature(readGeometry(f.geometry))
            feature.set(PKEY_PROPERTY, f.pkey)
            return feature
        })
        source.addFeatures(built)
        filling = false
        if (!hasAreal.value && built.some((feature) => isAreal(feature.getGeometry())))
            hasAreal.value = true
    }

    // Called on every change in the source, so it must decide on the feature
    // alone. Set silently and repainted through the source: marking a feature
    // dirty must not itself read as a change to the feature.
    function noteChange(feature: Feature<Geometry>) {
        // A polygon drawn in the session is as much a candidate for a hole as one
        // that came from the server.
        if (!hasAreal.value && isAreal(feature.getGeometry())) hasAreal.value = true
        if (filling || feature.get(DIRTY_PROPERTY)) return
        feature.set(DIRTY_PROPERTY, true, true)
        if (pkeyOf(feature) === null) createdCount.value += 1
        else changedCount.value += 1
        source.changed()
        // The dots are coloured by the same state, so the layer that draws them
        // has to be told too — it is not the one the change was announced on.
        if (focused === feature) focusSource.changed()
    }

    // The vertices a modification actually moved, merged into whatever the
    // feature already carried: one geometry is reshaped over several drags, and
    // each adds to what the save will write rather than replacing it.
    //
    // Merging is not appending, and that is the whole of it: a mark sits at the
    // position the vertex was dropped at, so the *next* drag on that same vertex
    // leaves the old mark behind, on a coordinate the geometry no longer holds —
    // a red dot floating where the vertex used to be. So every call first drops
    // the marks the geometry has stopped carrying. That also covers the two
    // cases nothing else would: a vertex deleted after being moved, and a whole
    // feature translated, after which no mark is on the geometry any more and
    // "these nodes moved" has stopped meaning anything.
    //
    // Set silently and repainted through the source, for the same reason
    // noteChange is: recording a move must not itself read as a change.
    // `arrived` are the coordinates the drag put there, `left` the ones it took
    // away. A reshape that added exactly one and removed exactly one is a node
    // that moved, and only that case gets a leader: one added and none removed
    // is a vertex inserted on a segment, which came from nowhere, and the
    // reverse is a deletion, which went nowhere.
    function noteMoved(feature: Feature<Geometry>, arrived: Coordinate[], left: Coordinate[]) {
        const previous = feature.get(MOVED_PROPERTY)
        const held = previous instanceof MultiPoint ? previous.getCoordinates() : []
        if (held.length === 0 && arrived.length === 0 && left.length === 0) return

        const current = new Set(
            (verticesOf(feature.getGeometry())?.getCoordinates() ?? []).map(coordinateKey)
        )
        const marks = new Map<string, Coordinate>()
        for (const coordinate of [...held, ...arrived]) {
            const id = coordinateKey(coordinate)
            if (current.has(id)) marks.set(id, coordinate)
        }

        if (arrived.length === 1 && left.length === 1) leaders.note(feature, left[0], arrived[0])
        leaders.retain(feature, new Set(marks.keys()))

        if (marks.size === 0) feature.unset(MOVED_PROPERTY, true)
        else feature.set(MOVED_PROPERTY, new MultiPoint([...marks.values()]), true)
        source.changed()
        // The marks are drawn by the focus layer too, which is not the one the
        // change was announced on (see focusStyle).
        if (focused === feature) focusSource.changed()
    }

    function changes(): LayerChanges {
        const updates = new Map<string, GeoJSON.Geometry>()
        const inserts: GeoJSON.Geometry[] = []
        for (const feature of source.getFeatures()) {
            const geometry = feature.getGeometry()
            if (!geometry) continue
            const pkey = pkeyOf(feature)
            if (pkey === null) inserts.push(writeGeometry(geometry))
            else if (feature.get(DIRTY_PROPERTY)) updates.set(pkey, writeGeometry(geometry))
        }
        return { updates, inserts }
    }

    // Gives back what the session is no longer using, on two grounds: an
    // untouched geometry the view has left, and — once those are gone — the
    // untouched ones reached longest ago, past MAX_RESTING. The second is what
    // bounds a session worked in place: reaching a geometry is a hover, hovering
    // is cheap, and a quarter of an hour of pointing over one neighbourhood
    // piles up features that never leave the view and that nothing would
    // otherwise release.
    //
    // A dirty feature is never released, in view or not: it carries unsaved
    // work. Nor is one that belongs to no row yet — it was drawn here, and the
    // server has no copy to give back — nor the one under the pointer, which is
    // the geometry the user is about to grab.
    function prune(): string[] {
        const m = map.value
        const size = m?.getSize()
        const extent = m && size ? m.getView().calculateExtent(size) : null
        const released: string[] = []

        function drop(feature: Feature<Geometry>, pkey: string) {
            if (focused === feature) focus(null)
            leaders.forget(feature)
            source.removeFeature(feature)
            released.push(pkey)
        }

        // Source order is the order the geometries were reached in, so the head of
        // this list is the oldest — no separate bookkeeping to keep true.
        const resting: Array<[Feature<Geometry>, string]> = []
        for (const feature of source.getFeatures()) {
            if (feature.get(DIRTY_PROPERTY)) continue
            const pkey = pkeyOf(feature)
            if (pkey === null) continue
            if (extent && !feature.getGeometry()?.intersectsExtent(extent)) {
                drop(feature, pkey)
                continue
            }
            if (focused !== feature) resting.push([feature, pkey])
        }
        for (let i = 0; i < resting.length - MAX_RESTING; i += 1) drop(...resting[i])

        return released
    }

    function remove() {
        const m = map.value
        if (!m || !added) return
        added = false
        m.removeLayer(layer)
        m.removeLayer(focusLayer)
        m.removeLayer(leaders.layer)
        leaders.clear()
        source.clear()
        focus(null)
        changedCount.value = 0
        createdCount.value = 0
        hasAreal.value = false
    }

    return {
        add,
        remove,
        source,
        solo,
        trackPointer,
        blur: () => focus(null),
        load,
        prune,
        noteChange,
        noteMoved,
        changes,
        dirtyCount,
        changedCount,
        createdCount,
        hasAreal
    }
}
