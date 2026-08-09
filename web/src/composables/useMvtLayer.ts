import { watch, type Ref } from 'vue'
import type Map from 'ol/Map'
import MVT from 'ol/format/MVT'
import { intersects } from 'ol/extent'
import VectorTileLayer from 'ol/layer/VectorTile'
import VectorTileSource from 'ol/source/VectorTile'
import { tileUrl } from '../client/geo'
import type { FilterCondition } from '../client/types'
import { mvtStyle } from '../utils/mapStyles'
import { viewExtent } from '../utils/projection'
import { xyzGrid } from '../utils/tileGrid'
import { useMapSelection, type FeatureSelection } from './useMapSelection'

// MVT rendering of a table: one vector-tile layer, styled by a function that
// answers per feature. Nothing has to be asked of the server to know which
// geometry types the column holds — the style reads the type off each
// feature, so a mixed column is one layer either way.
export function useMvtLayer(
    map: Ref<Map | null>,
    tableId: Ref<string>,
    bbox: Ref<string | undefined>,
    conditions: Ref<FilterCondition[]>,
    selection: FeatureSelection,
    version: Ref<number>
) {
    let layer: VectorTileLayer | null = null
    // Canvas: the style function is re-run per feature on every frame, so the
    // layer only has to be told to repaint.
    const picked = useMapSelection(map, selection, () => layer?.changed())

    function source(): VectorTileSource {
        // Stated, never defaulted: the default is a 512-pixel grid, which asks
        // the server for a different z — and so a different simplification —
        // than the endpoint is cut along (see utils/tileGrid).
        const grid = xyzGrid()
        const src = new VectorTileSource({
            // The tile carries the row's pkey; promoting it to the feature's id is
            // what lets a selection survive a tile reload, since a feature is
            // rebuilt from scratch on every pan.
            //
            // The server names the MVT layer after the tableId (back §4.7), so the
            // name is read off the same identifier the URL is built from — naming a
            // fixed one here would silently parse zero feature out of a full tile.
            format: new MVT({ idProperty: 'pkey', layers: [tableId.value] }),
            tileGrid: grid,
            url: tileUrl(tableId.value, conditions.value, version.value)
        })

        // Outside the table's own extent there is nothing to draw, so a tile that
        // could only come back empty is never asked for. The bound is put on the
        // requests and not on the layer: a layer extent also clips what is
        // painted, and a symbol is drawn around its coordinate — a point sitting
        // on the edge of its own table's bbox would be painted as a half circle,
        // cut along the bbox rather than along anything in the data.
        const extent = bbox.value ? viewExtent(bbox.value) : null
        if (extent) {
            const url = src.getTileUrlFunction()
            src.setTileUrlFunction((coord, pixelRatio, projection) =>
                intersects(grid.getTileCoordExtent(coord), extent)
                    ? url(coord, pixelRatio, projection)
                    : undefined
            )
        }
        return src
    }

    function activate() {
        const m = map.value
        if (!m || layer) return

        layer = new VectorTileLayer({
            source: source(),
            style: (feature) => mvtStyle(picked.isSelected(feature.getId()))
        })
        m.addLayer(layer)
        picked.activate(layer)
    }

    // A write changes what a tile holds and a filter changes which rows it
    // holds; both come back as a new URL, which is the only thing a tile cache
    // keyed by URL reacts to.
    function refreshTiles() {
        layer?.setSource(source())
    }

    function deactivate() {
        const m = map.value
        picked.deactivate()
        if (m && layer) m.removeLayer(layer)
        layer = null
    }

    // bbox is in the list because it bounds which tiles are asked for: a table
    // whose extent grew after an edit must be allowed to serve the tiles that
    // extent now covers.
    watch([conditions, version, bbox], () => {
        if (layer) refreshTiles()
    })

    // Which row is drawn under a pixel — how an edit session reaches the
    // geometry the pointer rests over (§8.8). What it answers is an identifier,
    // never a geometry: a tile's copy is simplified for its zoom and is never
    // what gets edited.
    return { activate, deactivate, pkeyAt: picked.pkeyAt }
}
