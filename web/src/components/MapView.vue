<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import maplibregl, { Map } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { getGeometryTypes, tileUrl } from "../client/geo";
import { getItems, type OgcFeature } from "../client/features";

const props = defineProps<{
  tableId: string;
  bbox?: string;
  wfsEnabled?: boolean;
}>();

const emit = defineEmits<{ selectionChange: [pkeys: string[]] }>();

const container = ref<HTMLDivElement | null>(null);
let map: Map | null = null;
const sourceId = `table-${props.tableId}`;
const sourceLayer = "layer";
const wfsSourceId = `wfs-${props.tableId}`;
const WFS_VIEWPORT_LIMIT = 100_000;
const selectedPkeys = new Set<string>();
let mvtLayerIds: string[] = [];
let mvtInteractiveLayerIds: string[] = [];
let onMvtMouseEnter: (() => void) | null = null;
let onMvtMouseLeave: (() => void) | null = null;
let onMvtClick: ((e: maplibregl.MapMouseEvent) => void) | null = null;
let onWfsMoveEnd: (() => void) | null = null;

// Metadata.bbox is reported as "minX,minY,maxX,maxY" in EPSG:4326 (the
// server reprojects it from the geometry column's storage SRID) so it
// can be handed to fitBounds directly.
function parseBbox(bbox: string): [number, number, number, number] | null {
  const parts = bbox.split(",").map(Number);
  if (parts.length !== 4 || parts.some(Number.isNaN)) return null;
  return parts as [number, number, number, number];
}

// A geometry column can mix types (règle 10) — classify by substring so
// POINT/MULTIPOINT, LINESTRING/MULTILINESTRING and POLYGON/MULTIPOLYGON
// all match, and only the layer(s) matching what's actually present get
// added, instead of always drawing fill+line+circle regardless of type.
function classifyGeometryTypes(types: string[]) {
  return {
    hasPolygon: types.some((t) => t.includes("POLYGON")),
    hasLine: types.some((t) => t.includes("LINESTRING")),
    hasPoint: types.some((t) => t.includes("POINT")),
  };
}

// A feature-state boolean expression: highlighted styling for selected
// features, base styling otherwise.
function selectedCase(selectedValue: unknown, baseValue: unknown): maplibregl.ExpressionSpecification {
  return ["case", ["boolean", ["feature-state", "selected"], false], selectedValue, baseValue] as maplibregl.ExpressionSpecification;
}

// Only fill/polygon layers register a dedicated hit target for click —
// the "-line" outline stroke is a few px wide and would make clicking a
// polygon's interior miss, so inspection always targets fill/circle first.
async function addMvtLayer(m: Map) {
  const bounds = props.bbox ? parseBbox(props.bbox) : null;
  m.addSource(sourceId, {
    type: "vector",
    tiles: [`${location.origin}${tileUrl(props.tableId)}`],
    // Promotes the "pkey" tile property to the MapLibre feature id, so
    // selection state (setFeatureState) can be tracked per row and
    // survives tile reloads instead of resetting on every pan/zoom.
    promoteId: { [sourceLayer]: "pkey" },
    ...(bounds ? { bounds } : {}),
  });

  const { types } = await getGeometryTypes(props.tableId).catch(() => ({ types: [] as string[] }));
  const { hasPolygon, hasLine, hasPoint } = classifyGeometryTypes(types);
  mvtLayerIds = [];
  mvtInteractiveLayerIds = [];

  if (hasPolygon) {
    const id = `${sourceId}-fill`;
    m.addLayer({
      id,
      type: "fill",
      source: sourceId,
      "source-layer": sourceLayer,
      paint: { "fill-color": selectedCase("#f59e0b", "#6366f1"), "fill-opacity": selectedCase(0.55, 0.35) },
    });
    mvtLayerIds.push(id);
    mvtInteractiveLayerIds.push(id);
  }
  if (hasPolygon || hasLine) {
    const id = `${sourceId}-line`;
    m.addLayer({
      id,
      type: "line",
      source: sourceId,
      "source-layer": sourceLayer,
      paint: { "line-color": selectedCase("#b45309", "#4338ca"), "line-width": selectedCase(3, 1.5) },
    });
    mvtLayerIds.push(id);
  }
  if (hasPoint) {
    const id = `${sourceId}-point`;
    m.addLayer({
      id,
      type: "circle",
      source: sourceId,
      "source-layer": sourceLayer,
      paint: {
        "circle-color": selectedCase("#f59e0b", "#4338ca"),
        "circle-radius": selectedCase(6, 4),
        "circle-stroke-color": "#ffffff",
        "circle-stroke-width": 1,
      },
    });
    mvtLayerIds.push(id);
    mvtInteractiveLayerIds.push(id);
  }

  if (mvtInteractiveLayerIds.length > 0) {
    onMvtMouseEnter = () => (m.getCanvas().style.cursor = "pointer");
    onMvtMouseLeave = () => (m.getCanvas().style.cursor = "");
    onMvtClick = (e) => toggleFeature(m, e.point, mvtInteractiveLayerIds);
    m.on("mouseenter", mvtInteractiveLayerIds, onMvtMouseEnter);
    m.on("mouseleave", mvtInteractiveLayerIds, onMvtMouseLeave);
    m.on("click", mvtInteractiveLayerIds, onMvtClick);
  }
}

function removeMvtLayer(m: Map) {
  if (mvtInteractiveLayerIds.length > 0 && onMvtMouseEnter && onMvtMouseLeave && onMvtClick) {
    m.off("mouseenter", mvtInteractiveLayerIds, onMvtMouseEnter);
    m.off("mouseleave", mvtInteractiveLayerIds, onMvtMouseLeave);
    m.off("click", mvtInteractiveLayerIds, onMvtClick);
  }
  for (const id of mvtLayerIds) {
    if (m.getLayer(id)) m.removeLayer(id);
  }
  if (m.getSource(sourceId)) m.removeSource(sourceId);
  mvtLayerIds = [];
  mvtInteractiveLayerIds = [];
  onMvtMouseEnter = null;
  onMvtMouseLeave = null;
  onMvtClick = null;
  selectedPkeys.clear();
}

function setFeatureSelected(m: Map, pkey: string, isSelected: boolean) {
  m.setFeatureState({ source: sourceId, sourceLayer, id: pkey }, { selected: isSelected });
}

// Each click toggles the clicked feature in/out of the selection — no
// modifier key needed, so the whole feature works on trackpad/touch too.
function toggleFeature(m: Map, point: maplibregl.PointLike, layerIds: string[]) {
  const feature = m.queryRenderedFeatures(point, { layers: layerIds })[0];
  if (feature?.id === undefined || feature.id === null) return;
  const pkey = String(feature.id);

  const isSelected = selectedPkeys.has(pkey);
  if (isSelected) {
    selectedPkeys.delete(pkey);
  } else {
    selectedPkeys.add(pkey);
  }
  setFeatureSelected(m, pkey, !isSelected);
  emit("selectionChange", Array.from(selectedPkeys));
}

// Lets the side panel drive selection back into the map (deselect one row,
// or clear everything) without duplicating the feature-state bookkeeping.
function deselect(pkey: string) {
  if (!map || !selectedPkeys.delete(pkey)) return;
  setFeatureSelected(map, pkey, false);
  emit("selectionChange", Array.from(selectedPkeys));
}

function clearSelection() {
  if (!map) return;
  for (const pkey of selectedPkeys) setFeatureSelected(map, pkey, false);
  selectedPkeys.clear();
  emit("selectionChange", []);
}

function addWfsLayer(m: Map) {
  m.addSource(wfsSourceId, { type: "geojson", data: { type: "FeatureCollection", features: [] } });
  m.addLayer({
    id: `${wfsSourceId}-fill`,
    type: "fill",
    source: wfsSourceId,
    filter: ["==", ["geometry-type"], "Polygon"],
    paint: { "fill-color": "#16a34a", "fill-opacity": 0.25 },
  });
  m.addLayer({
    id: `${wfsSourceId}-line`,
    type: "line",
    source: wfsSourceId,
    filter: ["in", ["geometry-type"], ["literal", ["Polygon", "LineString"]]],
    paint: { "line-color": "#15803d", "line-width": 2, "line-dasharray": [2, 1] },
  });
  m.addLayer({
    id: `${wfsSourceId}-point`,
    type: "circle",
    source: wfsSourceId,
    filter: ["==", ["geometry-type"], "Point"],
    paint: { "circle-color": "#16a34a", "circle-radius": 5, "circle-stroke-color": "#ffffff", "circle-stroke-width": 1 },
  });
}

function removeWfsLayer(m: Map) {
  if (onWfsMoveEnd) {
    m.off("moveend", onWfsMoveEnd);
    onWfsMoveEnd = null;
  }
  for (const id of [`${wfsSourceId}-fill`, `${wfsSourceId}-line`, `${wfsSourceId}-point`]) {
    if (m.getLayer(id)) m.removeLayer(id);
  }
  if (m.getSource(wfsSourceId)) m.removeSource(wfsSourceId);
}

let wfsRequestId = 0;

async function refreshWfsData(m: Map) {
  const source = m.getSource<maplibregl.GeoJSONSource>(wfsSourceId);
  if (!source) return;

  const requestId = ++wfsRequestId;
  const viewBounds = m.getBounds();
  const bbox: [number, number, number, number] = [
    viewBounds.getWest(),
    viewBounds.getSouth(),
    viewBounds.getEast(),
    viewBounds.getNorth(),
  ];
  const collection = await getItems(props.tableId, { bbox, limit: WFS_VIEWPORT_LIMIT }).catch(() => null);
  if (!collection || requestId !== wfsRequestId) return;

  const features = collection.features.filter((f): f is OgcFeature & { geometry: GeoJSON.Geometry } => f.geometry !== null);
  source.setData({ type: "FeatureCollection", features });
}

function enableWfsLayer(m: Map) {
  addWfsLayer(m);
  refreshWfsData(m);
  onWfsMoveEnd = () => refreshWfsData(m);
  m.on("moveend", onWfsMoveEnd);
}

defineExpose({ deselect, clearSelection });

onMounted(() => {
  if (!container.value) return;

  map = new maplibregl.Map({
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

  map.addControl(new maplibregl.NavigationControl(), "top-right");

  map.on("load", async () => {
    if (!map) return;

    if (props.wfsEnabled) {
      enableWfsLayer(map);
    } else {
      await addMvtLayer(map);
    }

    const bounds = props.bbox ? parseBbox(props.bbox) : null;
    if (bounds && map) {
      map.fitBounds(bounds, { padding: 32, animate: false });
    }
  });
});

onUnmounted(() => {
  map?.remove();
  map = null;
});

watch(
  () => props.bbox,
  (bbox) => {
    const bounds = bbox ? parseBbox(bbox) : null;
    if (bounds && map) {
      map.fitBounds(bounds, { padding: 32, animate: false });
    }
  },
);

// MVT and WFS are mutually exclusive on the map — switching modes tears
// down the previous mode's source/layers entirely rather than layering
// both, so there is only ever one rendering of the table at a time.
watch(
  () => props.wfsEnabled,
  (enabled) => {
    if (!map) return;
    if (enabled) {
      removeMvtLayer(map);
      enableWfsLayer(map);
    } else {
      removeWfsLayer(map);
      addMvtLayer(map);
    }
  },
);
</script>

<template>
  <div ref="container" class="h-full w-full rounded-lg" />
</template>
