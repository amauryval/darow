import Feature from 'ol/Feature'
import LineString from 'ol/geom/LineString'
import VectorLayer from 'ol/layer/Vector'
import VectorSource from 'ol/source/Vector'
import type Geometry from 'ol/geom/Geometry'
import type { Coordinate } from 'ol/coordinate'
import { coordinateKey, leaderStyle } from '../utils/drawStyles'

// What each node the session moved has done: a leader drawn from where the
// node stood before the user touched it to where it stands now. The red dot
// says *which* node moved; this says how far and which way, which is the
// question a save is about to answer for real.
//
// It is a layer of its own, above the edit layer and the vertices, for the
// same reason those are two layers: a leader is a whole annotation — line,
// origin ring, arrowhead — and drawing it out of the edited feature's style
// would rebuild it on every frame of every drag of that feature. Here it is
// built once, by the gesture that created it, and carried on its own feature
// (`Feature.setStyle`), so the layer needs no style function at all.
interface Leader {
    owner: Feature<Geometry>
    from: Coordinate
    to: Coordinate
    drawn: Feature<Geometry>
}

export function useMoveLeaders(zIndex: number) {
    const source = new VectorSource()
    const layer = new VectorLayer({ source, zIndex })
    // Held here rather than as feature properties: `from` and `to` are read
    // back to chain one drag onto the next, and a coordinate taken out of an
    // untyped property bag has to be re-narrowed every time it is.
    let leaders: Leader[] = []

    function draw(owner: Feature<Geometry>, from: Coordinate, to: Coordinate) {
        const drawn = new Feature(new LineString([from, to]))
        drawn.setStyle(leaderStyle(from, to))
        source.addFeature(drawn)
        leaders.push({ owner, from, to, drawn })
    }

    function drop(leader: Leader) {
        source.removeFeature(leader.drawn)
        leaders = leaders.filter((held) => held !== leader)
    }

    // A node dragged twice has **one** leader, not two: the second drag starts
    // where the first ended, and what the user is checking before they save is
    // where the node was before they touched it at all — not where it was
    // halfway through. So a move whose origin is another leader's arrival
    // extends that leader instead of adding one beside it.
    function note(owner: Feature<Geometry>, from: Coordinate, to: Coordinate) {
        const chained = leaders.find(
            (leader) => leader.owner === owner && coordinateKey(leader.to) === coordinateKey(from)
        )
        const origin = chained ? chained.from : from
        if (chained) drop(chained)
        // A node brought back to where it started has not moved, and an arrow of
        // length zero has no direction to point in.
        if (coordinateKey(origin) === coordinateKey(to)) return
        draw(owner, origin, to)
    }

    // Keeps only the leaders whose arrival the owner still carries. It is what
    // clears a leader when its node is deleted, dragged back onto its origin,
    // or carried off by a whole-feature move — the caller knows which
    // coordinates survived, this does not have to ask the geometry again.
    function retain(owner: Feature<Geometry>, arrivals: Set<string>) {
        for (const leader of [...leaders]) {
            if (leader.owner === owner && !arrivals.has(coordinateKey(leader.to))) drop(leader)
        }
    }

    // The owner is gone from the session — released, or the session closed.
    function forget(owner: Feature<Geometry>) {
        for (const leader of [...leaders]) if (leader.owner === owner) drop(leader)
    }

    function clear() {
        source.clear()
        leaders = []
    }

    return { layer, note, retain, forget, clear }
}
