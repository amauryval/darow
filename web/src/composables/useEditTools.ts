import { computed, ref, type Ref } from 'vue'
import type Map from 'ol/Map'
import Draw from 'ol/interaction/Draw'
import type Interaction from 'ol/interaction/Interaction'
import Snap from 'ol/interaction/Snap'
import type VectorSource from 'ol/source/Vector'
import { useHoleTool } from './useHoleTool'

// The shapes a session draws. A geometry already in the layer is reshaped
// whatever its type; only a new one has to be told what it is.
export type DrawShape = 'Point' | 'LineString' | 'Polygon'

// How close to another vertex a placed one has to land to be pulled onto it,
// in screen pixels — a tolerance in meters would mean one thing at the
// parcel's zoom and another at the district's.
const SNAP_PIXELS = 12

// Which tool the edit bar has armed, and nothing else: the shape being drawn,
// the hole being cut, the snapping that corrects both. The session's own
// lifecycle — the layer, the Modify, the teardown — is useMapDraw's.
//
// The two are split because every arming path has to respect the Snap's
// place: it must handle the pointer before the interactions it corrects, so
// nothing may be added above it. Keeping that rule in one small state machine
// is what stops the next tool from forgetting it.
export function useEditTools(map: Ref<Map | null>, source: VectorSource) {
    const shape = ref<DrawShape | null>(null)
    // Two independent switches, because they answer two different questions.
    //
    // Snapping is about *placing*: a vertex put down within tolerance of a
    // neighbour is pulled exactly onto it. Topology is about *moving*: whether
    // a coordinate two geometries already share is one vertex or two. Snapping
    // is how a boundary becomes shared; topology is what happens to it
    // afterwards. Wanting to snap a new shape onto its neighbours without
    // dragging those neighbours around is an ordinary thing to want.
    const snapping = ref(true)
    const topological = ref(true)
    // Moving a whole geometry rather than one of its vertices. It is a mode and
    // not a gesture, because the two cannot share the pointer: a press near a
    // boundary is either "grab this vertex" or "grab this shape", and with both
    // interactions armed the vertex always wins and the move never happens. The
    // caller swaps the `Modify` for a `Translate` on it (useMapDraw).
    const moving = ref(false)
    let draw: Draw | null = null
    let snap: Snap | null = null

    // The hole tool carves whichever polygon it was drawn inside, so it is
    // handed the source rather than one chosen feature. Its Draw is snapped
    // like any other, so it goes in through the same door.
    const hole = useHoleTool(map, source, { attach: addUnderSnap, onCut: () => source.changed() })

    // Whether a shape is being drawn at all — what tells the bar that taking
    // back a point means something (§8.2).
    const drawing = computed(() => shape.value !== null || hole.cutting.value)

    // Snapping is what makes two features share an exact boundary — and a
    // boundary shared exactly is what lets one drag move both sides at once,
    // so it is also what makes topological editing possible at all.
    //
    // The Snap is built once per session and then left alone. Taking it off the
    // map clears the segment index it keeps over every feature in view, and
    // putting it back rebuilds it — a freeze the length of the whole layer, and
    // the old code paid it on every tool picked and every toggle flipped.
    function ensureSnap(m: Map) {
        if (!snapping.value || snap) return
        snap = new Snap({ source, pixelTolerance: SNAP_PIXELS })
        // Added last, so it is handled first: OpenLayers walks its interactions
        // backwards, and a Snap that ran after the Draw would correct a
        // coordinate already used.
        m.addInteraction(snap)
    }

    // The one way to add an interaction the Snap has to correct: below it, so
    // the index it built survives. Everything a session arms goes through here.
    function addUnderSnap(m: Map, interaction: Interaction) {
        const interactions = m.getInteractions()
        const at = snap ? interactions.getArray().indexOf(snap) : -1
        interactions.insertAt(at === -1 ? interactions.getLength() : at, interaction)
    }

    function stopDraw(m: Map) {
        if (draw) m.removeInteraction(draw)
        draw = null
        shape.value = null
    }

    // Picking a shape arms the drawing of a new feature; picking it again, or
    // picking another, disarms or replaces it. Outside that the map is in
    // reshaping mode, which is what a session spends most of its time doing.
    function setShape(next: DrawShape | null) {
        const m = map.value
        if (!m) return
        hole.stop()
        stopDraw(m)
        // Drawing and moving are two things to do with the same drag: arming one
        // disarms the other rather than leaving the map answering neither (§8.2).
        if (next !== null) moving.value = false
        if (next !== null) {
            shape.value = next
            const started = new Draw({ source, type: next })
            // A drawn shape is a row that does not exist yet; it stays on the map,
            // editable like the others, until the session is saved.
            started.on('drawend', () => stopDraw(m))
            draw = started
            addUnderSnap(m, started)
        }
    }

    function toggleHole() {
        const m = map.value
        if (!m) return
        stopDraw(m)
        moving.value = false
        hole.toggle()
    }

    // Arming the move drops whatever was being drawn: the drag it takes over is
    // the one that was placing vertices.
    function toggleMoving() {
        const m = map.value
        moving.value = !moving.value
        if (!moving.value || !m) return
        hole.stop()
        stopDraw(m)
    }

    function toggleSnapping() {
        const m = map.value
        snapping.value = !snapping.value
        if (!m) return
        if (snapping.value) ensureSnap(m)
        else if (snap) {
            m.removeInteraction(snap)
            snap = null
        }
    }

    // Only the state flips here: the caller rebuilds its Modify around it and
    // adds it back through addUnderSnap.
    function toggleTopology() {
        topological.value = !topological.value
    }

    // Takes back the last click of whichever shape is being drawn — a new
    // feature, or a hole being cut into an existing one.
    function undo() {
        if (hole.active()) hole.removeLastPoint()
        else draw?.removeLastPoint()
    }

    // A shape still being drawn is dropped rather than closed: a shape is
    // finished with a double-click, and what half of one was meant to be is not
    // for the save button to decide.
    function abort() {
        draw?.abortDrawing()
        hole.stop()
    }

    // Called when the session opens: nothing is armed yet, but the Snap that
    // corrects the Modify already has to be there.
    function arm() {
        if (map.value) ensureSnap(map.value)
    }

    function teardown() {
        const m = map.value
        hole.stop()
        if (m) {
            for (const interaction of [draw, snap])
                if (interaction) m.removeInteraction(interaction)
        }
        draw = null
        snap = null
        shape.value = null
        moving.value = false
    }

    return {
        shape,
        snapping,
        topological,
        moving,
        cuttingHole: hole.cutting,
        drawing,
        addUnderSnap,
        setShape,
        toggleHole,
        toggleMoving,
        toggleSnapping,
        toggleTopology,
        undo,
        abort,
        arm,
        teardown
    }
}
