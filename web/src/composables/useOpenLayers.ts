import { onMounted, onUnmounted, shallowRef, watch, type Ref } from 'vue'
import Map from 'ol/Map'
import View from 'ol/View'
import TileLayer from 'ol/layer/Tile'
import OSM from 'ol/source/OSM'
import { viewExtent } from '../utils/projection'

// Map lifecycle: creation (raster OSM basemap, default controls), framing on
// the initial load and on every bbox change, destruction.
//
// map is a shallowRef — OpenLayers mutates the instance internally through
// its own imperative API, so Vue only needs to track its identity, not
// deep-observe it. onReady fires as soon as the map exists: unlike a style-
// driven engine there is nothing to wait for, a layer can be added the
// moment the map is constructed.
const FIT_PADDING = 32

export function useOpenLayers(
    container: Ref<HTMLDivElement | null>,
    bbox: Ref<string | undefined>,
    onReady: (map: Map) => void
) {
    const map = shallowRef<Map | null>(null)
    // A fit needs the viewport's size, which only exists once the map has been
    // laid out — the first bbox usually arrives before that, so it waits here
    // rather than being silently dropped.
    let pending: string | undefined

    function fitBbox(bboxStr?: string) {
        const m = map.value
        const extent = bboxStr ? viewExtent(bboxStr) : null
        if (!extent) return
        if (!m || !m.getSize()) {
            pending = bboxStr
            return
        }
        pending = undefined
        m.getView().fit(extent, {
            padding: [FIT_PADDING, FIT_PADDING, FIT_PADDING, FIT_PADDING],
            duration: 0
        })
    }

    onMounted(() => {
        if (!container.value) return

        const m = new Map({
            target: container.value,
            // The OSM source carries its own attribution, and the default control
            // set renders it alongside zoom and rotate — §8.8 asks for both.
            layers: [new TileLayer({ source: new OSM() })],
            view: new View({ center: [0, 0], zoom: 1 })
        })
        map.value = m

        m.on('postrender', () => {
            if (pending !== undefined) fitBbox(pending)
        })

        fitBbox(bbox.value)
        onReady(m)
    })

    onUnmounted(() => {
        // Detaching the target is what releases the listeners OpenLayers bound on
        // the DOM element; dropping the reference alone would leak them.
        map.value?.setTarget(undefined)
        map.value = null
    })

    watch(bbox, (b) => fitBbox(b))

    return { map, fitBbox }
}
