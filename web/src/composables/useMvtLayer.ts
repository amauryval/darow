import { watch, type Ref } from "vue";
import type Map from "ol/Map";
import MVT from "ol/format/MVT";
import VectorTileLayer from "ol/layer/VectorTile";
import VectorTileSource from "ol/source/VectorTile";
import { tileUrl } from "../client/geo";
import type { FilterCondition } from "../client/types";
import { mvtStyle } from "../utils/mapStyles";
import { viewExtent } from "../utils/projection";
import { xyzGrid } from "../utils/tileGrid";
import { useMapSelection, type FeatureSelection } from "./useMapSelection";

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
  version: Ref<number>,
) {
  let layer: VectorTileLayer | null = null;
  // Canvas: the style function is re-run per feature on every frame, so the
  // layer only has to be told to repaint.
  const picked = useMapSelection(map, selection, () => layer?.changed());

  function source(): VectorTileSource {
    return new VectorTileSource({
      // The tile carries the row's pkey; promoting it to the feature's id is
      // what lets a selection survive a tile reload, since a feature is
      // rebuilt from scratch on every pan.
      format: new MVT({ idProperty: "pkey", layers: ["layer"] }),
      // Stated, never defaulted: the default is a 512-pixel grid, which asks
      // the server for a different z — and so a different simplification —
      // than the endpoint is cut along (see utils/tileGrid).
      tileGrid: xyzGrid(),
      url: tileUrl(tableId.value, conditions.value, version.value),
    });
  }

  function activate() {
    const m = map.value;
    if (!m || layer) return;

    const extent = bbox.value ? viewExtent(bbox.value) : null;
    layer = new VectorTileLayer({
      source: source(),
      // Outside the table's own extent there is nothing to draw: bounding the
      // layer is what keeps the map from asking the server for empty tiles.
      ...(extent ? { extent } : {}),
      style: (feature) => mvtStyle(picked.isSelected(feature.getId())),
    });
    m.addLayer(layer);
    picked.activate(layer);
  }

  // A write changes what a tile holds and a filter changes which rows it
  // holds; both come back as a new URL, which is the only thing a tile cache
  // keyed by URL reacts to.
  function refreshTiles() {
    layer?.setSource(source());
  }

  function deactivate() {
    const m = map.value;
    picked.deactivate();
    if (m && layer) m.removeLayer(layer);
    layer = null;
  }

  watch([conditions, version], () => {
    if (layer) refreshTiles();
  });

  return { activate, deactivate };
}
