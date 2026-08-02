import { onMounted, onUnmounted, shallowRef, watch, type Ref } from "vue";
import maplibregl, { type Map } from "maplibre-gl";
import { parseBbox } from "../utils/bbox";

// Map lifecycle: creation (raster OSM basemap, navigation control),
// fitBounds on the initial load and on every bbox change, destruction.
// map is a shallowRef — MapLibre mutates the instance internally through
// its own imperative API, so Vue only needs to track its identity, not
// deep-observe it. onLoad fires once the style has finished loading, which
// is when a layer composable (useMvtLayer/useWfsLayer) can safely add its
// source and layers.
export function useMapLibre(container: Ref<HTMLDivElement | null>, bbox: Ref<string | undefined>, onLoad: (map: Map) => void) {
  const map = shallowRef<Map | null>(null);

  function flyToBbox(bboxStr?: string) {
    const bounds = bboxStr ? parseBbox(bboxStr) : null;
    if (bounds && map.value) map.value.fitBounds(bounds, { padding: 32, animate: false });
  }

  onMounted(() => {
    if (!container.value) return;

    const m = new maplibregl.Map({
      container: container.value,
      style: {
        version: 8,
        sources: {
          osm: {
            type: "raster",
            tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
            tileSize: 256,
            attribution: "© OpenStreetMap contributors",
          },
        },
        layers: [{ id: "osm", type: "raster", source: "osm" }],
      },
      center: [0, 0],
      zoom: 1,
    });
    m.addControl(new maplibregl.NavigationControl(), "top-right");
    map.value = m;

    m.on("load", () => {
      flyToBbox(bbox.value);
      onLoad(m);
    });
  });

  onUnmounted(() => {
    map.value?.remove();
    map.value = null;
  });

  watch(bbox, (b) => flyToBbox(b));

  return { map, flyToBbox };
}
