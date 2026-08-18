import { computed, ref, watch, type Ref } from 'vue'
import type Map from 'ol/Map'
import type Feature from 'ol/Feature'
import type Geometry from 'ol/geom/Geometry'
import type { Extent } from 'ol/extent'
import WebGLVectorLayer from 'ol/layer/WebGLVector'
import VectorSource from 'ol/source/Vector'
import { tile as tileStrategy } from 'ol/loadingstrategy'
import { getAllItems } from '../client/features'
import type { FilterCondition } from '../client/types'
import { wfsFlatStyle } from '../utils/mapStyles'
import { dataBbox, readFeatures } from '../utils/projection'
import { tileSimplifyTolerance, xyzGrid } from '../utils/tileGrid'
import { useMapSelection, type FeatureSelection } from './useMapSelection'
import { useToast } from './useToast'

// WFS/OGC-Features rendering of a table: real GeoJSON features rather than
// tiles, read from the OGC API - Features items endpoint — the standard
// route, the one an outside client would use, and the only one that counts
// what it matches (`numberMatched`), which is what lets a dense area load its
// pages several at a time instead of one after another.
//
// The XYZ grid is only the loading strategy's, never the query's: OpenLayers
// decides which tiles the view is missing, remembers the ones already loaded
// across a pan, and asks again only for what it does not have. Each of those
// tiles is one bbox query.
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
    // A refresh empties the source; a tile still paging at that moment would
    // drop its remaining pages into it, as a leftover of a version the map no
    // longer shows. Each load carries the generation it started under and
    // stops as soon as that generation is over.
    let generation = 0
    // How many tile queries are in flight, so the map can say it is working
    // instead of leaving a pan silent until its first page lands (§8.8). A
    // count, not a flag: the strategy asks for several tiles at once and the
    // indicator has to stay on until the last of them answers.
    const pending = ref(0)

    // A feature straddling a tile boundary is returned by every tile's bbox
    // query; adding it twice would draw it twice and let one click select it
    // and the next deselect the copy.
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
        // A tile of the grid's edge covers ground the projection cannot
        // express; what is left of it after the world is cut out is what the
        // server can be asked about, and nothing at all is a loaded tile
        // rather than a failed one — no pan will ever make it answer.
        const bbox = dataBbox(extent)
        if (!bbox) {
            success([])
            return
        }

        pending.value += 1
        try {
            await loadPages(source, bbox, resolution, success, failure)
        } finally {
            pending.value -= 1
        }
    }

    async function loadPages(
        source: VectorSource,
        bbox: [number, number, number, number],
        resolution: number,
        success: (features: Feature<Geometry>[]) => void,
        failure: () => void
    ) {
        // The zoom is still the grid's, since the simplification tolerance is
        // a function of it — but it says nothing about which rows are asked
        // for any more, only how finely they come back.
        const z = grid.getZForResolution(resolution)
        const loadedUnder = generation
        const added: Feature<Geometry>[] = []
        // No pageSize override: the client's own chunk size is a UI choice and
        // the server clamps it to what the operator allows anyway.
        const { complete } = await getAllItems(tableId.value, {
            bbox,
            simplify: tileSimplifyTolerance(z),
            filter: conditions.value,
            // Each page is drawn as it lands, and they land out of order: a
            // dense area fills in instead of holding the view blank until its
            // last page answers.
            onPage: (features) => {
                if (loadedUnder !== generation) return
                const read = readFeatures({
                    type: 'FeatureCollection',
                    features: features.flatMap((f) =>
                        f.geometry ? [{ ...f, geometry: f.geometry }] : []
                    )
                })
                const fresh = unseen(source, read)
                source.addFeatures(fresh)
                added.push(...fresh)
            }
        })

        // The source was emptied under this load: the engine has already
        // forgotten the tile and asks for it again, so this one just ends.
        if (loadedUnder !== generation) {
            failure()
            return
        }

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
        generation += 1
        layer?.getSource()?.refresh()
    }

    function deactivate() {
        const m = map.value
        picked.deactivate()
        if (m && layer) m.removeLayer(layer)
        layer = null
        warnedIncomplete = false
        generation += 1
    }

    watch([conditions, version], () => {
        if (layer) refreshData()
    })

    return {
        activate,
        deactivate,
        refreshData,
        pkeyAt: picked.pkeyAt,
        loading: computed(() => pending.value > 0)
    }
}
