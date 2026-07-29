<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import maplibregl, { Map } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { getGeometryTypes, tileUrl } from "../client/geo";

const props = defineProps<{
  tableId: string;
  bbox?: string;
}>();

const emit = defineEmits<{ selectionChange: [pkeys: string[]] }>();

const container = ref<HTMLDivElement | null>(null);
let map: Map | null = null;
const sourceId = `table-${props.tableId}`;
const sourceLayer = "layer";
const selectedPkeys = new Set<string>();

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
async function addTableLayer(m: Map): Promise<string[]> {
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
  const interactiveLayerIds: string[] = [];

  if (hasPolygon) {
    const id = `${sourceId}-fill`;
    m.addLayer({
      id,
      type: "fill",
      source: sourceId,
      "source-layer": sourceLayer,
      paint: { "fill-color": selectedCase("#f59e0b", "#6366f1"), "fill-opacity": selectedCase(0.55, 0.35) },
    });
    interactiveLayerIds.push(id);
  }
  if (hasPolygon || hasLine) {
    m.addLayer({
      id: `${sourceId}-line`,
      type: "line",
      source: sourceId,
      "source-layer": sourceLayer,
      paint: { "line-color": selectedCase("#b45309", "#4338ca"), "line-width": selectedCase(3, 1.5) },
    });
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
    interactiveLayerIds.push(id);
  }
  return interactiveLayerIds;
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

  map.on("load", () => {
    if (!map) return;
    addTableLayer(map).then((layerIds) => {
      if (!map || layerIds.length === 0) return;
      const m = map;
      m.on("mouseenter", layerIds, () => (m.getCanvas().style.cursor = "pointer"));
      m.on("mouseleave", layerIds, () => (m.getCanvas().style.cursor = ""));
      m.on("click", layerIds, (e) => toggleFeature(m, e.point, layerIds));
    });

    const bounds = props.bbox ? parseBbox(props.bbox) : null;
    if (bounds) {
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
</script>

<template>
  <div ref="container" class="h-full w-full rounded-lg" />
</template>
