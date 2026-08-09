import { watch, type Ref } from 'vue'
import type Map from 'ol/Map'
import type Feature from 'ol/Feature'
import type Geometry from 'ol/geom/Geometry'
import type { Extent } from 'ol/extent'
import { getCenter } from 'ol/extent'
import WebGLVectorLayer from 'ol/layer/WebGLVector'
import VectorSource from 'ol/source/Vector'
import { tile as tileStrategy } from 'ol/loadingstrategy'
import { getAllTileFeatures } from '../client/geo'
import type { FilterCondition } from '../client/types'
import { wfsFlatStyle } from '../utils/mapStyles'
import { readFeatures } from '../utils/projection'
import { tileSimplifyTolerance, xyzGrid } from '../utils/tileGrid'
import { useMapSelection, type FeatureSelection } from './useMapSelection'
import { useToast } from './useToast'

// WFS/OGC-Features rendering of a table: real GeoJSON features rather than
// tiles, loaded over the same XYZ grid the MVT endpoint is cut along. The
// grid is the loading strategy's: OpenLayers decides which tiles the view is
// missing, remembers the ones already loaded across a pan, and asks again
// only for what it does not have.
const grid = xyzGrid()

export function useWfsLayer(
    map: Ref<Map | null>,
    tableId: Ref<string>,
    conditions: Ref<FilterCondition[]>,
    selection: FeatureSelection,
    version: Ref<number>
) {
    const toast = useToast()
    let layer: WebGLVectorLayer<VectorSource> | null = null
    // The GPU compiles the style once, so a selection change is a new style
    // rather than a repaint — see wfsFlatStyle.
    const picked = useMapSelection(map, selection, (pkeys) => layer?.setStyle(wfsFlatStyle(pkeys)))
    // Warns at most once per run of failures rather than on every tile — reset
    // as soon as a tile comes back complete, so a later, separate failure
    // still warns again.
    let warnedIncomplete = false

    // A feature straddling a tile boundary is returned by every tile it
    // intersects; adding it twice would draw it twice and let one click select
    // it and the next deselect the copy.
    function unseen(source: VectorSource, features: Feature<Geometry>[]): Feature<Geometry>[] {
        return features.filter((feature) => {
            const id = feature.getId()
            return id === undefined || source.getFeatureById(id) === null
        })
    }

    async function loadTile(
        source: VectorSource,
        extent: Extent,
        resolution: number,
        success: (features: Feature<Geometry>[]) => void,
        failure: () => void
    ) {
        const z = grid.getZForResolution(resolution)
        const [, x, y] = grid.getTileCoordForCoordAndZ(getCenter(extent), z)
        // No pageSize override: client/geo.ts's own chunk size is a UI choice,
        // deliberately independent of the operator's defaultFeaturesLimit setting —
        // a REST API ceiling, not a page-fetch tuning knob.
        const { features, complete } = await getAllTileFeatures(tableId.value, z, x, y, {
            simplify: tileSimplifyTolerance(z),
            filter: conditions.value
        })

        const read = readFeatures({
            type: 'FeatureCollection',
            features: features.flatMap((f) => (f.geometry ? [{ ...f, geometry: f.geometry }] : []))
        })
        const added = unseen(source, read)
        source.addFeatures(added)

        if (complete) {
            warnedIncomplete = false
            success(added)
            return
        }
        // What did load stays on screen, but the tile is not marked loaded: the
        // next pan asks for it again instead of leaving a hole no gesture fills.
        failure()
        if (!warnedIncomplete) {
            warnedIncomplete = true
            toast.error(
                "Certaines tuiles n'ont pas pu être chargées — les données affichées peuvent être incomplètes."
            )
        }
    }

    function activate() {
        const m = map.value
        if (!m || layer) return

        const source = new VectorSource({
            strategy: tileStrategy(grid),
            loader: (extent, resolution, _projection, success, failure) => {
                if (success && failure) loadTile(source, extent, resolution, success, failure)
            }
        })
        layer = new WebGLVectorLayer({ source, style: wfsFlatStyle(selection.selectedPkeys.value) })
        m.addLayer(layer)
        picked.activate(layer)
    }

    // A filter changes which rows a tile holds and a write changes their
    // geometry: both invalidate every tile already loaded, so the source
    // forgets all of them rather than growing a second version alongside.
    function refreshData() {
        layer?.getSource()?.refresh()
    }

    function deactivate() {
        const m = map.value
        picked.deactivate()
        if (m && layer) m.removeLayer(layer)
        layer = null
        warnedIncomplete = false
    }

    watch([conditions, version], () => {
        if (layer) refreshData()
    })

    return { activate, deactivate, refreshData, pkeyAt: picked.pkeyAt }
}
