import { describe, expect, it } from 'vitest'
import Feature from 'ol/Feature'
import LineString from 'ol/geom/LineString'
import MultiPoint from 'ol/geom/MultiPoint'
import Point from 'ol/geom/Point'
import Circle from 'ol/style/Circle'
import RegularShape from 'ol/style/RegularShape'
import {
    DIRTY_PROPERTY,
    EDIT_LEGEND,
    MOVED_PROPERTY,
    PKEY_PROPERTY,
    coordinateKey,
    editStyle,
    focusStyle,
    handleStyle,
    leaderStyle,
    verticesOf
} from './drawStyles'

function line(): LineString {
    return new LineString([
        [0, 0],
        [1, 1],
        [2, 0]
    ])
}

function feature(properties: Record<string, unknown>): Feature {
    const f = new Feature({ geometry: line() })
    f.setProperties(properties)
    return f
}

describe('coordinateKey', () => {
    it('identifies a coordinate by its two first ordinates', () => {
        expect(coordinateKey([1.5, -2])).toBe('1.5|-2')
    })

    it('tells two neighbouring coordinates apart', () => {
        expect(coordinateKey([1, 2])).not.toBe(coordinateKey([2, 1]))
    })
})

describe('verticesOf', () => {
    it('answers every vertex of the geometry', () => {
        expect(verticesOf(line())?.getCoordinates()).toEqual([
            [0, 0],
            [1, 1],
            [2, 0]
        ])
    })

    it('answers nothing for a geometry that is not a simple one', () => {
        expect(verticesOf(undefined)).toBeUndefined()
    })

    it('caches the answer, so a style function does not rebuild it per frame', () => {
        const geometry = line()

        expect(verticesOf(geometry)).toBe(verticesOf(geometry))
    })

    it('rebuilds the vertices once the geometry actually moved', () => {
        const geometry = line()
        const before = verticesOf(geometry)
        geometry.setCoordinates([
            [0, 0],
            [5, 5]
        ])

        const after = verticesOf(geometry)

        expect(after).not.toBe(before)
        expect(after?.getCoordinates()).toEqual([
            [0, 0],
            [5, 5]
        ])
    })
})

describe('editStyle', () => {
    it('draws a reached, untouched geometry in the neutral resting tone', () => {
        const styles = editStyle(feature({ [PKEY_PROPERTY]: 'pk-1' }))

        expect(styles).toHaveLength(1)
        expect(styles[0].getStroke()?.getColor()).toBe('#475569')
    })

    it('draws a modified geometry in amber, heavier', () => {
        const styles = editStyle(feature({ [PKEY_PROPERTY]: 'pk-1', [DIRTY_PROPERTY]: true }))

        expect(styles[0].getStroke()?.getColor()).toBe('#b45309')
        expect(styles[0].getStroke()?.getWidth()).toBe(3.5)
    })

    it('draws a shape belonging to no row yet in violet, whether or not it was dragged', () => {
        expect(editStyle(feature({})).at(0)?.getStroke()?.getColor()).toBe('#5b21b6')
        expect(
            editStyle(feature({ [DIRTY_PROPERTY]: true }))
                .at(0)
                ?.getStroke()
                ?.getColor()
        ).toBe('#5b21b6')
    })

    it('adds the moved nodes to the body of a feature carrying some', () => {
        const moved = new MultiPoint([[1, 1]])

        const styles = editStyle(
            feature({ [PKEY_PROPERTY]: 'pk-1', [DIRTY_PROPERTY]: true, [MOVED_PROPERTY]: moved })
        )

        expect(styles).toHaveLength(2)
    })

    it('answers a pre-built style rather than allocating per frame', () => {
        const props = { [PKEY_PROPERTY]: 'pk-1' }

        expect(editStyle(feature(props))).toBe(editStyle(feature(props)))
    })
})

describe('focusStyle', () => {
    it('draws the hovered geometry’s vertices in its own state', () => {
        const resting = focusStyle(feature({ [PKEY_PROPERTY]: 'pk-1' }))
        const changed = focusStyle(feature({ [PKEY_PROPERTY]: 'pk-1', [DIRTY_PROPERTY]: true }))
        const created = focusStyle(feature({}))

        expect(resting).not.toBe(changed)
        expect(created).not.toBe(changed)
    })

    it('repeats the moved nodes, which this layer would otherwise bury', () => {
        expect(focusStyle(feature({ [PKEY_PROPERTY]: 'pk-1' }))).toHaveLength(2)
    })
})

describe('leaderStyle', () => {
    it('draws a dashed leader, a ring on the origin and an arrowhead on the arrival', () => {
        const [stroke, origin, head] = leaderStyle([0, 0], [0, 10])

        expect(stroke.getStroke()?.getLineDash()).toEqual([5, 4])
        expect(origin.getGeometry()).toBeInstanceOf(Point)
        expect(origin.getImage()).toBeInstanceOf(Circle)
        expect(head.getImage()).toBeInstanceOf(RegularShape)
    })

    it('rotates the arrowhead as an azimuth — clockwise from north', () => {
        const north = leaderStyle([0, 0], [0, 10])[2].getImage()?.getRotation()
        const east = leaderStyle([0, 0], [10, 0])[2].getImage()?.getRotation()

        expect(north).toBeCloseTo(0, 6)
        expect(east).toBeCloseTo(Math.PI / 2, 6)
    })
})

describe('EDIT_LEGEND', () => {
    it('names the three states the session colours, in the bar’s order', () => {
        expect(EDIT_LEGEND.map((entry) => entry.label)).toEqual([
            'Sommet déplacé',
            'Géométrie modifiée',
            'Nouvelle'
        ])
    })
})

describe('handleStyle', () => {
    it('draws the vertex under the pointer heavier than a resting one', () => {
        const image = handleStyle.getImage()

        expect(image instanceof Circle && image.getRadius()).toBe(6)
        expect(image instanceof Circle && image.getStroke()?.getWidth()).toBe(3)
    })
})
