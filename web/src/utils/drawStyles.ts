import Circle from 'ol/style/Circle'
import Fill from 'ol/style/Fill'
import RegularShape from 'ol/style/RegularShape'
import Stroke from 'ol/style/Stroke'
import Style from 'ol/style/Style'
import MultiPoint from 'ol/geom/MultiPoint'
import Point from 'ol/geom/Point'
import SimpleGeometry from 'ol/geom/SimpleGeometry'
import type { FeatureLike } from 'ol/Feature'
import type Geometry from 'ol/geom/Geometry'
import type RenderFeature from 'ol/render/Feature'
import type { Coordinate } from 'ol/coordinate'

// The colours an edit session is drawn in, and what it does **not** colour.
//
// Being editable is not a state worth a colour. Every row of the layer is a
// candidate, the session simply has not been pointed at all of them yet, and
// painting "you may grab this" over a rendering the user is reading turns the
// whole screen into a status display. So a reached, untouched geometry is
// drawn in a neutral slate that merely retraces the boundary already there —
// it is the vertices under the pointer that say what can be grabbed (§8.8).
//
// What is coloured is what the session has *done*, because that is what a
// save is about to write. Amber is the colour a pending write carries
// everywhere in the application, and the one the grid tints a retyped cell
// with; violet marks a shape that belongs to no row yet, since saving it is
// an insert and not an update — and it is the colour the empty row it raised
// is drawn in, on the other half of the screen.
const RESTING = '#64748b'
const RESTING_DARK = '#475569'
const CHANGED = '#f59e0b'
const CHANGED_DARK = '#b45309'
const CREATED = '#7c3aed'
const CREATED_DARK = '#5b21b6'
// The vertices a drag actually moved, told apart from the rest of the
// geometry carrying them: on a boundary of ten thousand points, "this row
// changed" says almost nothing and "these three points changed" says all of
// it. Red-orange rather than the feature's amber, so a moved node stands out
// against the shape it belongs to instead of melting into it.
const MOVED = '#dc2626'

// The property names the edit layer tags its features with, kept next to the
// style that has to read them on every frame.
export const DIRTY_PROPERTY = 'dirty'
export const PKEY_PROPERTY = 'pkey'
// The vertices of this feature that a modification moved, held as the
// MultiPoint the style hands straight back — a style function runs per
// feature per frame and must not build geometry.
export const MOVED_PROPERTY = 'moved'

// Every vertex of a geometry, rebuilt only when that geometry actually moved.
// A style function runs once per feature per frame, and the vertices of one
// dragged geometry are wanted on every frame of the drag — so the answer is
// cached against the geometry's revision rather than recomputed, and one
// stale MultiPoint per geometry is all it ever holds.
interface CachedVertices {
    revision: number
    points: MultiPoint | undefined
}
const vertexCache = new WeakMap<Geometry, CachedVertices>()

// The identity of a coordinate, for the one question anyone asks about one:
// does this geometry still carry it? Written once here, next to `verticesOf`,
// because a mark placed on a vertex and the geometry it is compared against
// have to agree on what "the same point" means.
export function coordinateKey(coordinate: Coordinate): string {
    return `${coordinate[0]}|${coordinate[1]}`
}

export function verticesOf(geometry: Geometry | RenderFeature | undefined): MultiPoint | undefined {
    if (!(geometry instanceof SimpleGeometry)) return undefined
    const revision = geometry.getRevision()
    const cached = vertexCache.get(geometry)
    if (cached && cached.revision === revision) return cached.points
    const flat = geometry.getFlatCoordinates()
    const stride = geometry.getStride()
    const points: Coordinate[] = []
    for (let i = 0; i + 1 < flat.length; i += stride) points.push([flat[i], flat[i + 1]])
    const built = new MultiPoint(points)
    vertexCache.set(geometry, { revision, points: built })
    return built
}

// What an editable surface is drawn as: its boundary and its vertices, not a
// second filled surface over the one the render mode already draws
// underneath. A fill is a tessellation per geometry per frame, and it was
// paid to hide the very rendering the session is meant to be compared
// against.
//
// It is not dropped outright, and that is the whole reason for the value:
// OpenLayers hit-tests what a style actually paints, so a polygon with no
// fill can only be reached on its outline — and reaching a polygon is how
// its vertices are shown and how the move tool grabs it.
const BODY_ALPHA = 0.02

// A shape drawn in the session is the exception: nothing is rendered under it
// — it belongs to no row yet, so no tile and no WFS feature carries it — and
// an outline over the basemap alone reads as a hole rather than as a surface.
// It is the one geometry whose fill is not paid twice, so it is the one that
// gets one.
const CREATED_ALPHA = 0.3

// A touched geometry is drawn heavier as well as recoloured: §8.7 asks that
// information never rest on colour alone, and a stroke width survives a
// colour-blind reading where a hue does not.
function bodyStyle(color: string, dark: string, width: number, alpha = BODY_ALPHA): Style[] {
    return [
        new Style({
            fill: new Fill({ color: rgba(color, alpha) }),
            stroke: new Stroke({ color: dark, width }),
            image: new Circle({ radius: 4, fill: new Fill({ color: dark }) })
        })
    ]
}

// The dots that say a limit can be grabbed, drawn from whatever the feature's
// geometry is at that frame — so they follow it through a drag rather than
// marking where it used to be.
function vertexStyle(dark: string): Style {
    return new Style({
        geometry: (feature) => verticesOf(feature.getGeometry()),
        image: new Circle({
            radius: 4.5,
            fill: new Fill({ color: '#ffffff' }),
            stroke: new Stroke({ color: dark, width: 1.5 })
        })
    })
}

function rgba(hex: string, alpha: number): string {
    const value = Number.parseInt(hex.slice(1), 16)
    return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`
}

// Built once and handed back as they are: a style function is called for
// every feature of every frame, so it must not allocate.
const RESTING_STYLE = bodyStyle(RESTING, RESTING_DARK, 1.5)
const CHANGED_STYLE = bodyStyle(CHANGED, CHANGED_DARK, 3.5)
const CREATED_STYLE = bodyStyle(CREATED, CREATED_DARK, 3.5, CREATED_ALPHA)
const RESTING_VERTICES = vertexStyle(RESTING_DARK)
const CHANGED_VERTICES = vertexStyle(CHANGED_DARK)
const CREATED_VERTICES = vertexStyle(CREATED_DARK)

// The nodes a drag moved, drawn wherever they are and not only under the
// pointer: they are the one thing on screen a save is about to write, so they
// stay visible once the pointer has gone elsewhere. It costs nothing that the
// hover dots do not already cost — the set is bounded by the gestures the
// user actually made, not by what the view holds.
const MOVED_VERTICES = new Style({
    geometry: (feature) => {
        const moved = feature.get(MOVED_PROPERTY)
        return moved instanceof MultiPoint ? moved : undefined
    },
    image: new Circle({
        radius: 5,
        fill: new Fill({ color: MOVED }),
        stroke: new Stroke({ color: '#ffffff', width: 1.5 })
    })
})

// What the map's legend names, so the states are readable without
// decoding the colours (§8.7). The order is the one the bar shows. Being
// editable is not in it — it is not a state, and nothing on screen announces
// it.
export const EDIT_LEGEND = [
    { tone: MOVED, label: 'Sommet déplacé' },
    { tone: CHANGED_DARK, label: 'Géométrie modifiée' },
    { tone: CREATED_DARK, label: 'Nouvelle' }
]

// A feature with no pkey belongs to no row yet, whether or not it has been
// dragged since it was drawn: it is new, and stays new until it is saved.
function created(feature: FeatureLike): boolean {
    return feature.get(PKEY_PROPERTY) === undefined
}

// Built once per body style, since a feature carrying moved nodes needs both
// its own outline and those nodes in the same answer.
const CHANGED_WITH_MOVED = [...CHANGED_STYLE, MOVED_VERTICES]
const CREATED_WITH_MOVED = [...CREATED_STYLE, MOVED_VERTICES]

export function editStyle(feature: FeatureLike): Style[] {
    const moved = feature.get(MOVED_PROPERTY) !== undefined
    if (created(feature)) return moved ? CREATED_WITH_MOVED : CREATED_STYLE
    if (!feature.get(DIRTY_PROPERTY)) return RESTING_STYLE
    return moved ? CHANGED_WITH_MOVED : CHANGED_STYLE
}

// The vertices of the one geometry the pointer rests over, drawn in that
// geometry's own state so the dots never contradict the shape under them.
//
// This is a layer of its own, holding that single feature, and that is the
// point: vertices for the whole session's worth of geometry meant thousands
// of points rebuilt on every frame of every pan and every drag. Kept apart,
// a change of what is hovered repaints one geometry instead of a whole view.
// The moved nodes are repeated here, and they have to be: this layer draws
// above the one carrying them, so a hover would otherwise bury them under the
// plain dots of the geometry they belong to.
const RESTING_FOCUS = [RESTING_VERTICES, MOVED_VERTICES]
const CHANGED_FOCUS = [CHANGED_VERTICES, MOVED_VERTICES]
const CREATED_FOCUS = [CREATED_VERTICES, MOVED_VERTICES]

export function focusStyle(feature: FeatureLike): Style[] {
    if (created(feature)) return CREATED_FOCUS
    return feature.get(DIRTY_PROPERTY) ? CHANGED_FOCUS : RESTING_FOCUS
}

// The path a node took: a dashed leader from where it stood before the user
// touched it to where it stands now, a hollow ring on the origin and an
// arrowhead on the arrival. It says the one thing the red dot cannot — how
// far, and which way.
//
// Built per leader and set on the leader itself (`Feature.setStyle`), never
// answered by a style function: the arrowhead's rotation is that leader's own
// direction, so it cannot be one shared, pre-built style, and a style
// function returning a fresh one would rebuild it on every frame. A leader is
// created by a gesture, which is when this runs.
//
// A `RegularShape` and not an icon: it is rotatable, sized in pixels — so the
// head stays the same size at every zoom while the leader it caps is in map
// coordinates — and it needs no image to embed.
export function leaderStyle(from: Coordinate, to: Coordinate): Style[] {
    return [
        new Style({ stroke: new Stroke({ color: MOVED, width: 1.5, lineDash: [5, 4] }) }),
        new Style({
            geometry: new Point(from),
            image: new Circle({
                radius: 3.5,
                fill: new Fill({ color: '#ffffff' }),
                stroke: new Stroke({ color: MOVED, width: 1.5 })
            })
        }),
        new Style({
            geometry: new Point(to),
            image: new RegularShape({
                points: 3,
                radius: 6,
                // OpenLayers rotates clockwise from north and the triangle points
                // north unrotated, so the direction is read as an azimuth — atan2 of
                // (dx, dy), not the usual (dy, dx) angle from the x axis.
                rotation: Math.atan2(to[0] - from[0], to[1] - from[1]),
                fill: new Fill({ color: MOVED }),
                stroke: new Stroke({ color: '#ffffff', width: 1 })
            })
        })
    ]
}

// The handle under the pointer — the one that will move if the user drags
// now, drawn heavier than a resting vertex so the target is unambiguous.
export const handleStyle = new Style({
    image: new Circle({
        radius: 6,
        fill: new Fill({ color: '#ffffff' }),
        stroke: new Stroke({ color: RESTING_DARK, width: 3 })
    })
})
