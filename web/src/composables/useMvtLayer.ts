import { watch, type Ref } from "vue";
import type { Map, MapMouseEvent, VectorTileSource } from "maplibre-gl";
import { getGeometryTypes, tileUrl } from "../client/geo";
import type { FilterCondition } from "../client/types";
import { parseBbox } from "../utils/bbox";
import { buildMvtLayers } from "../utils/mvtStyle";

export interface FeatureSelection {
  selectedPkeys: Ref<string[]>;
  toggle: (pkey: string) => void;
}

// MVT rendering of a table: source + layers picked by the geometry types
// actually present, click-to-toggle selection reflected as MapLibre
// feature-state. selection is the single source of truth
// (useFeatureSelection): this composable only reflects it onto the map and
// reports clicks back via toggle(), it never keeps its own selected-set.
export function useMvtLayer(
  map: Ref<Map | null>,
  tableId: Ref<string>,
  bbox: Ref<string | undefined>,
  conditions: Ref<FilterCondition[]>,
  selection: FeatureSelection,
) {
  const sourceId = `table-${tableId.value}`;
  const sourceLayer = "layer";
  let active = false;
  let layerIds: string[] = [];
  let interactiveLayerIds: string[] = [];
  let onMouseEnter: (() => void) | null = null;
  let onMouseLeave: (() => void) | null = null;
  let onClick: ((e: MapMouseEvent) => void) | null = null;
  let trackedSelected = new Set<string>();

  function setFeatureSelected(m: Map, pkey: string, isSelected: boolean) {
    m.setFeatureState({ source: sourceId, sourceLayer, id: pkey }, { selected: isSelected });
  }

  function syncSelection(m: Map) {
    const current = new Set(selection.selectedPkeys.value);
    for (const pkey of trackedSelected) if (!current.has(pkey)) setFeatureSelected(m, pkey, false);
    for (const pkey of current) if (!trackedSelected.has(pkey)) setFeatureSelected(m, pkey, true);
    trackedSelected = current;
  }

  async function activate() {
    const m = map.value;
    if (!m) return;
    active = true;

    const bounds = bbox.value ? parseBbox(bbox.value) : null;
    m.addSource(sourceId, {
      type: "vector",
      tiles: [`${location.origin}${tileUrl(tableId.value, conditions.value)}`],
      // Promotes the "pkey" tile property to the MapLibre feature id, so
      // selection state (setFeatureState) can be tracked per row and
      // survives tile reloads instead of resetting on every pan/zoom.
      promoteId: { [sourceLayer]: "pkey" },
      ...(bounds ? { bounds } : {}),
    });

    // Best-effort: a failure here degrades to no MVT layers added rather
    // than blocking activation — the tile source itself was already added
    // above and still renders once the map issues its own tile requests,
    // so losing only the type-driven styling is the tolerated silence
    // spec/front/spec.md §0.13 allows for a tile-related display load.
    const { types } = await getGeometryTypes(tableId.value).catch(() => ({ types: [] as string[] }));
    if (!active) return;

    layerIds = [];
    interactiveLayerIds = [];
    for (const { layer, interactive } of buildMvtLayers(sourceId, sourceLayer, types)) {
      m.addLayer(layer);
      layerIds.push(layer.id);
      if (interactive) interactiveLayerIds.push(layer.id);
    }

    if (interactiveLayerIds.length > 0) {
      onMouseEnter = () => (m.getCanvas().style.cursor = "pointer");
      onMouseLeave = () => (m.getCanvas().style.cursor = "");
      // Each click toggles the clicked feature in/out of the selection —
      // no modifier key needed, so the whole feature works on trackpad/
      // touch too.
      onClick = (e) => {
        const feature = m.queryRenderedFeatures(e.point, { layers: interactiveLayerIds })[0];
        if (feature?.id === undefined || feature.id === null) return;
        selection.toggle(String(feature.id));
      };
      m.on("mouseenter", interactiveLayerIds, onMouseEnter);
      m.on("mouseleave", interactiveLayerIds, onMouseLeave);
      m.on("click", interactiveLayerIds, onClick);
    }

    syncSelection(m);
  }

  function refreshTiles() {
    const m = map.value;
    const source = m?.getSource<VectorTileSource>(sourceId);
    source?.setTiles([`${location.origin}${tileUrl(tableId.value, conditions.value)}`]);
  }

  function deactivate() {
    const m = map.value;
    active = false;
    if (m) {
      if (interactiveLayerIds.length > 0 && onMouseEnter && onMouseLeave && onClick) {
        m.off("mouseenter", interactiveLayerIds, onMouseEnter);
        m.off("mouseleave", interactiveLayerIds, onMouseLeave);
        m.off("click", interactiveLayerIds, onClick);
      }
      for (const id of layerIds) if (m.getLayer(id)) m.removeLayer(id);
      if (m.getSource(sourceId)) m.removeSource(sourceId);
    }
    layerIds = [];
    interactiveLayerIds = [];
    onMouseEnter = null;
    onMouseLeave = null;
    onClick = null;
    trackedSelected = new Set();
  }

  watch(conditions, () => {
    if (active) refreshTiles();
  });
  watch(selection.selectedPkeys, () => {
    if (active && map.value) syncSelection(map.value);
  });

  return { activate, deactivate };
}
