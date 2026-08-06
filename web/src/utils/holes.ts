import MultiPolygon from 'ol/geom/MultiPolygon'
import Polygon from 'ol/geom/Polygon'
import type Geometry from 'ol/geom/Geometry'
import type { Coordinate } from 'ol/coordinate'

// Carving a hole is adding an interior ring, not a set operation: the ring
// the user drew becomes part of the polygon it was drawn inside, and every
// vertex of it stays editable afterwards like any other. Which also means a
// hole is undone the way it was made — by dragging its vertices, or by
// removing them until the ring is no longer one.
//
// Answers null when the drawn ring has no polygon to belong to: outside the
// shape, straddling its boundary, inside a hole it already has, or on a
// geometry that has no interior to speak of. A ring only half inside would
// otherwise be appended anyway and written back as an invalid polygon, with
// nothing on screen saying so.
export function punchHole(target: Geometry, hole: Polygon): Geometry | null {
    const ring = hole.getCoordinates()[0]
    if (!ring || ring.length < 4) return null

    if (target instanceof Polygon) {
        return holds(target, ring) ? new Polygon([...target.getCoordinates(), ring]) : null
    }

    if (target instanceof MultiPolygon) {
        // A hole belongs to exactly one part — the one it was drawn inside.
        const parts = target.getPolygons()
        const index = parts.findIndex((part) => holds(part, ring))
        if (index === -1) return null
        return new MultiPolygon(
            parts.map((part, i) =>
                i === index ? [...part.getCoordinates(), ring] : part.getCoordinates()
            )
        )
    }

    return null
}

// Every vertex of the ring, not just the first: a ring straddling the limit
// has vertices on both sides, and a ring drawn inside an existing hole has
// none inside the polygon's interior — intersectsCoordinate already answers
// false there, since a polygon's interior excludes its own holes.
function holds(part: Polygon, ring: Coordinate[]): boolean {
    return ring.every((coordinate) => part.intersectsCoordinate(coordinate))
}

export function isAreal(geometry: Geometry | null | undefined): boolean {
    return geometry instanceof Polygon || geometry instanceof MultiPolygon
}
