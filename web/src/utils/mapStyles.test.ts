import { describe, expect, it } from 'vitest'
import Circle from 'ol/style/Circle'
import { mvtStyle, wfsFlatStyle } from './mapStyles'
import type { FlatStyle, FlatStyleLike } from 'ol/style/flat'

// FlatStyleLike also covers a list of styles and a rule set; wfsFlatStyle
// answers a single one, and the tests read its properties by name.
function single(style: FlatStyleLike): FlatStyle {
    if (Array.isArray(style)) throw new Error('un style unique était attendu')
    return style
}

describe('mvtStyle', () => {
    it('reads the tiles in indigo when nothing is selected', () => {
        const style = mvtStyle(false)

        expect(style.getFill()?.getColor()).toBe('rgba(99, 102, 241, 0.35)')
        expect(style.getStroke()?.getColor()).toBe('#4338ca')
    })

    it('reads a selected feature in the amber the table pins its rows with', () => {
        const style = mvtStyle(true)

        expect(style.getFill()?.getColor()).toBe('rgba(245, 158, 11, 0.55)')
        expect(style.getStroke()?.getColor()).toBe('#b45309')
    })

    it('carries fill, stroke and image at once, so a mixed-type table needs one style', () => {
        const image = mvtStyle(false).getImage()

        expect(image).toBeInstanceOf(Circle)
        expect(image instanceof Circle && image.getRadius()).toBe(4)
    })

    it('answers the same instance for a given state — the style is not rebuilt per frame', () => {
        expect(mvtStyle(true)).toBe(mvtStyle(true))
        expect(mvtStyle(false)).not.toBe(mvtStyle(true))
    })
})

describe('wfsFlatStyle', () => {
    it('bakes the selection into the compiled style as an id test', () => {
        const style = single(wfsFlatStyle(['pk-1', 'pk-2']))

        expect(style['fill-color']).toEqual([
            'case',
            ['in', ['id'], ['literal', ['pk-1', 'pk-2']]],
            'rgba(245, 158, 11, 0.35)',
            'rgba(22, 163, 74, 0.25)'
        ])
    })

    it('expresses no selection as a plain false — an empty list has no type to check', () => {
        const style = single(wfsFlatStyle([]))

        expect(style['fill-color']).toEqual(['case', false, expect.any(String), expect.any(String)])
        expect(style['stroke-width']).toEqual(['case', false, 3, 2])
    })

    it('renders the features in green, apart from the indigo of the tiles', () => {
        const style = single(wfsFlatStyle([]))

        expect(style['stroke-color']).toEqual(['case', false, '#b45309', '#15803d'])
        expect(style['circle-fill-color']).toEqual(['case', false, '#f59e0b', '#16a34a'])
    })
})
