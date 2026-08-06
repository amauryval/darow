import Circle from 'ol/style/Circle'
import Fill from 'ol/style/Fill'
import Stroke from 'ol/style/Stroke'
import Style from 'ol/style/Style'
import type { FlatStyleLike } from 'ol/style/flat'
import type { ExpressionValue } from 'ol/expr/expression'

// The colours a table is read in, and the only place they are written. One
// Style carries fill, stroke and image at once: OpenLayers applies the fill
// and stroke to areal and linear geometries and the image to points, so a
// mixed-type column needs one style, not one layer per type.
//
// The two render modes are deliberately not the same colour — indigo for the
// tiles, green for the features — so it is always readable which one is on.
// Selected is amber in both, the same amber the table pins its selected rows
// with, so the two halves of one selection read alike.
const SELECTED = '#f59e0b'
const SELECTED_DARK = '#b45309'

function style(
    fill: string,
    fillOpacity: number,
    stroke: string,
    width: number,
    radius: number,
    lineDash?: number[]
) {
    return new Style({
        fill: new Fill({ color: rgba(fill, fillOpacity) }),
        stroke: new Stroke({ color: stroke, width, lineDash }),
        image: new Circle({
            radius,
            fill: new Fill({ color: fill }),
            stroke: new Stroke({ color: '#ffffff', width: 1 })
        })
    })
}

// OpenLayers takes a colour and an alpha as one value, where the style spec
// took two — the hex stays the single written form and the alpha is applied
// here.
function rgba(hex: string, alpha: number): string {
    const value = Number.parseInt(hex.slice(1), 16)
    return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`
}

const MVT_BASE = style('#6366f1', 0.35, '#4338ca', 1.5, 4)
const MVT_SELECTED = style(SELECTED, 0.55, SELECTED_DARK, 3, 6)

export function mvtStyle(selected: boolean): Style {
    return selected ? MVT_SELECTED : MVT_BASE
}

// The WFS layer renders on the GPU, which does not run a style function per
// feature — it compiles one style into a shader. So the selection is not read
// at draw time but baked into the style, and a selection change recompiles
// it. That is affordable because a selection changes on a click and not on a
// frame; it is also why the MVT layer, whose selection is the same one,
// cannot follow (see limitations LIM-F6).
//
// An empty haystack has no type for the `in` operator to check against, so no
// selection is expressed as a plain false rather than as an empty list.
export function wfsFlatStyle(selectedPkeys: string[]): FlatStyleLike {
    const isSelected: ExpressionValue =
        selectedPkeys.length > 0 ? ['in', ['id'], ['literal', selectedPkeys]] : false
    return {
        'fill-color': ['case', isSelected, rgba(SELECTED, 0.35), rgba('#16a34a', 0.25)],
        'stroke-color': ['case', isSelected, SELECTED_DARK, '#15803d'],
        'stroke-width': ['case', isSelected, 3, 2],
        'stroke-line-dash': [6, 3],
        'circle-radius': ['case', isSelected, 6, 5],
        'circle-fill-color': ['case', isSelected, SELECTED, '#16a34a'],
        'circle-stroke-color': '#ffffff',
        'circle-stroke-width': 1
    }
}
