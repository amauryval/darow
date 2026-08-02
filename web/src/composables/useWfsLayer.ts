import { watch, type Ref } from "vue";
import type { GeoJSONSource, Map } from "maplibre-gl";
import type { OgcFeature } from "../client/features";
import type { FilterCondition } from "../client/types";
import { useTiledFeatures } from "./useTiledFeatures";

// WFS/OGC-Features rendering of a table: a single GeoJSON source split by a
// ["geometry-type"] filter expression into fill/line/circle layers — no
// per-type server round-trip needed, unlike MVT. Data loads on moveend,
// tile by tile, reusing useTiledFeatures' cache. No selection support: WFS
// mode has no click handler, unlike useMvtLayer.
export function useWfsLayer(map: Ref<Map | null>, tableId: Ref<string>, conditions: Ref<FilterCondition[]>) {
  const sourceId = `wfs-${tableId.value}`;
  const tiledFeatures = useTiledFeatures();
  let active = false;
  let onMoveEnd: (() => void) | null = null;
  let requestId = 0;

  function addSourceAndLayers(m: Map) {
    m.addSource(sourceId, { type: "geojson", data: { type: "FeatureCollection", features: [] } });
    m.addLayer({
      id: `${sourceId}-fill`,
      type: "fill",
      source: sourceId,
      filter: ["==", ["geometry-type"], "Polygon"],
      paint: { "fill-color": "#16a34a", "fill-opacity": 0.25 },
    });
    m.addLayer({
      id: `${sourceId}-line`,
      type: "line",
      source: sourceId,
      filter: ["in", ["geometry-type"], ["literal", ["Polygon", "LineString"]]],
      paint: { "line-color": "#15803d", "line-width": 2, "line-dasharray": [2, 1] },
    });
    m.addLayer({
      id: `${sourceId}-point`,
      type: "circle",
      source: sourceId,
      filter: ["==", ["geometry-type"], "Point"],
      paint: { "circle-color": "#16a34a", "circle-radius": 5, "circle-stroke-color": "#ffffff", "circle-stroke-width": 1 },
    });
  }

  function removeSourceAndLayers(m: Map) {
    for (const id of [`${sourceId}-fill`, `${sourceId}-line`, `${sourceId}-point`]) {
      if (m.getLayer(id)) m.removeLayer(id);
    }
    if (m.getSource(sourceId)) m.removeSource(sourceId);
  }

  async function refreshData() {
    const m = map.value;
    const maybeSource = m?.getSource<GeoJSONSource>(sourceId);
    if (!m || !maybeSource) return;
    // Rebound to a definitely-assigned const: TypeScript doesn't retain
    // narrowing of a captured variable across the nested render() closure
    // below.
    const source = maybeSource;

    const thisRequestId = ++requestId;
    const viewBounds = m.getBounds();
    const bbox: [number, number, number, number] = [
      viewBounds.getWest(),
      viewBounds.getSouth(),
      viewBounds.getEast(),
      viewBounds.getNorth(),
    ];
    const zoom = Math.max(0, Math.floor(m.getZoom()));

    // A GeoJSON source re-indexes its whole feature set on every setData
    // call (synchronously, on the main thread) — calling it once per tile
    // as they land in a burst freezes the map. Coalescing to one setData
    // per animation frame keeps the progressive-loading feel without the
    // reindex storm.
    let renderScheduled = false;
    let latest: OgcFeature[] | null = null;

    function render(loaded: OgcFeature[]) {
      if (thisRequestId !== requestId) return;
      latest = loaded;
      if (renderScheduled) return;
      renderScheduled = true;
      requestAnimationFrame(() => {
        renderScheduled = false;
        if (thisRequestId !== requestId || !latest) return;
        const features = latest.filter((f): f is OgcFeature & { geometry: GeoJSON.Geometry } => f.geometry !== null);
        source.setData({ type: "FeatureCollection", features });
      });
    }

    await tiledFeatures.loadVisible(tableId.value, bbox, zoom, conditions.value, render).catch(() => null);
  }

  function activate() {
    const m = map.value;
    if (!m) return;
    active = true;
    addSourceAndLayers(m);
    refreshData();
    onMoveEnd = () => refreshData();
    m.on("moveend", onMoveEnd);
  }

  function deactivate() {
    const m = map.value;
    active = false;
    if (m && onMoveEnd) m.off("moveend", onMoveEnd);
    onMoveEnd = null;
    if (m) removeSourceAndLayers(m);
  }

  watch(conditions, () => {
    if (active) refreshData();
  });

  return { activate, deactivate };
}
