import { watch, type Ref } from "vue";
import type { ExpressionSpecification, GeoJSONSource, Map, MapMouseEvent } from "maplibre-gl";
import type { OgcFeature } from "../client/features";
import type { FilterCondition } from "../client/types";
import type { FeatureSelection } from "./useMvtLayer";
import { useTiledFeatures } from "./useTiledFeatures";
import { useToast } from "./useToast";

const SELECTED_COLOR = "#f59e0b";

// WFS/OGC-Features rendering of a table: a single GeoJSON source split by a
// ["geometry-type"] filter expression into fill/line/circle layers — no
// per-type server round-trip needed, unlike MVT. Data loads on moveend,
// tile by tile, reusing useTiledFeatures' cache. Selection is the same
// selection the table and the MVT mode share: this composable reflects it
// onto the map and reports clicks back through toggle(), it never keeps its
// own selected set.
export function useWfsLayer(
  map: Ref<Map | null>,
  tableId: Ref<string>,
  conditions: Ref<FilterCondition[]>,
  selection: FeatureSelection,
  version: Ref<number>,
) {
  const sourceId = `wfs-${tableId.value}`;
  const layerIds = [`${sourceId}-fill`, `${sourceId}-line`, `${sourceId}-point`];
  const tiledFeatures = useTiledFeatures();
  const toast = useToast();
  let active = false;
  let onMoveEnd: (() => void) | null = null;
  let onMouseEnter: (() => void) | null = null;
  let onMouseLeave: (() => void) | null = null;
  let onClick: ((e: MapMouseEvent) => void) | null = null;
  let requestId = 0;
  let trackedSelected = new Set<string>();
  // Warns at most once per run of failures rather than on every moveend —
  // reset as soon as a load comes back complete, so a later, separate
  // failure still warns again.
  let warnedIncomplete = false;

  // A selected feature is drawn in the same amber the table pins its
  // selected rows with, so both halves of one selection read alike.
  function selectedOr(color: string): ExpressionSpecification {
    return ["case", ["boolean", ["feature-state", "selected"], false], SELECTED_COLOR, color];
  }

  function addSourceAndLayers(m: Map) {
    m.addSource(sourceId, {
      type: "geojson",
      data: { type: "FeatureCollection", features: [] },
      // A pkey is a UUID, and MapLibre only keeps a GeoJSON feature's own
      // id when it is a number — promoting the property is what makes
      // per-feature selection state survive a reload of the source.
      promoteId: "pkey",
    });
    m.addLayer({
      id: `${sourceId}-fill`,
      type: "fill",
      source: sourceId,
      filter: ["==", ["geometry-type"], "Polygon"],
      paint: { "fill-color": selectedOr("#16a34a"), "fill-opacity": 0.25 },
    });
    m.addLayer({
      id: `${sourceId}-line`,
      type: "line",
      source: sourceId,
      filter: ["in", ["geometry-type"], ["literal", ["Polygon", "LineString"]]],
      paint: { "line-color": selectedOr("#15803d"), "line-width": 2, "line-dasharray": [2, 1] },
    });
    m.addLayer({
      id: `${sourceId}-point`,
      type: "circle",
      source: sourceId,
      filter: ["==", ["geometry-type"], "Point"],
      paint: {
        "circle-color": selectedOr("#16a34a"),
        "circle-radius": 5,
        "circle-stroke-color": "#ffffff",
        "circle-stroke-width": 1,
      },
    });
  }

  function removeSourceAndLayers(m: Map) {
    for (const id of layerIds) {
      if (m.getLayer(id)) m.removeLayer(id);
    }
    if (m.getSource(sourceId)) m.removeSource(sourceId);
  }

  function setFeatureSelected(m: Map, pkey: string, isSelected: boolean) {
    m.setFeatureState({ source: sourceId, id: pkey }, { selected: isSelected });
  }

  function syncSelection(m: Map) {
    if (!m.getSource(sourceId)) return;
    const current = new Set(selection.selectedPkeys.value);
    for (const pkey of trackedSelected) if (!current.has(pkey)) setFeatureSelected(m, pkey, false);
    for (const pkey of current) if (!trackedSelected.has(pkey)) setFeatureSelected(m, pkey, true);
    trackedSelected = current;
  }

  // The OGC feature carries its pkey as the feature id; MapLibre reads it
  // from the properties instead (see promoteId), so it is copied there
  // before the source ever sees the feature.
  function withPkeyProperty(feature: OgcFeature & { geometry: GeoJSON.Geometry }) {
    return { ...feature, properties: { ...feature.properties, pkey: feature.id } };
  }

  function bindInteractions(m: Map) {
    onMouseEnter = () => (m.getCanvas().style.cursor = "pointer");
    onMouseLeave = () => (m.getCanvas().style.cursor = "");
    // One click toggles the clicked feature in and out of the selection,
    // with no modifier key — the same gesture the MVT mode answers to.
    onClick = (e) => {
      const feature = m.queryRenderedFeatures(e.point, { layers: layerIds })[0];
      const pkey = feature?.properties?.pkey;
      if (typeof pkey !== "string") return;
      selection.toggle(pkey);
    };
    m.on("mouseenter", layerIds, onMouseEnter);
    m.on("mouseleave", layerIds, onMouseLeave);
    m.on("click", layerIds, onClick);
  }

  function unbindInteractions(m: Map) {
    if (onMouseEnter) m.off("mouseenter", layerIds, onMouseEnter);
    if (onMouseLeave) m.off("mouseleave", layerIds, onMouseLeave);
    if (onClick) m.off("click", layerIds, onClick);
    onMouseEnter = null;
    onMouseLeave = null;
    onClick = null;
  }

  async function refreshData() {
    const m = map.value;
    const maybeSource = m?.getSource<GeoJSONSource>(sourceId);
    if (!m || !maybeSource) return;
    // Rebound to a definitely-assigned const: TypeScript doesn't retain
    // narrowing of a captured variable across the nested render() closure
    // below.
    const source = maybeSource;
    const mapInstance = m;

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
        const features = latest
          .filter((f): f is OgcFeature & { geometry: GeoJSON.Geometry } => f.geometry !== null)
          .map(withPkeyProperty);
        source.setData({ type: "FeatureCollection", features });
        // Feature state is keyed by id and reapplied on every reload, so
        // freshly rendered features come back highlighted too.
        trackedSelected = new Set();
        syncSelection(mapInstance);
      });
    }

    const result = await tiledFeatures.loadVisible(tableId.value, bbox, zoom, conditions.value, render).catch(() => null);
    if (thisRequestId !== requestId) return;

    if (!result || !result.complete) {
      if (!warnedIncomplete) {
        warnedIncomplete = true;
        toast.error("Certaines tuiles n'ont pas pu être chargées — les données affichées peuvent être incomplètes.");
      }
    } else {
      warnedIncomplete = false;
    }
  }

  function activate() {
    const m = map.value;
    if (!m) return;
    active = true;
    addSourceAndLayers(m);
    bindInteractions(m);
    refreshData();
    onMoveEnd = () => refreshData();
    m.on("moveend", onMoveEnd);
  }

  function deactivate() {
    const m = map.value;
    active = false;
    if (m && onMoveEnd) m.off("moveend", onMoveEnd);
    onMoveEnd = null;
    if (m) {
      unbindInteractions(m);
      removeSourceAndLayers(m);
    }
    trackedSelected = new Set();
  }

  watch(conditions, () => {
    if (active) refreshData();
  });
  watch(version, () => {
    tiledFeatures.invalidate();
    if (active) refreshData();
  });
  watch(selection.selectedPkeys, () => {
    if (active && map.value) syncSelection(map.value);
  });

  return { activate, deactivate, refreshData };
}
