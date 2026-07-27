<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import maplibregl, { Map } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { getGeometryTypes, tileUrl } from "../client/geo";

const props = defineProps<{
  tableId: string;
  bbox?: string;
}>();

const container = ref<HTMLDivElement | null>(null);
let map: Map | null = null;

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

async function addTableLayer(m: Map) {
  const sourceId = `table-${props.tableId}`;
  const bounds = props.bbox ? parseBbox(props.bbox) : null;
  m.addSource(sourceId, {
    type: "vector",
    tiles: [`${location.origin}${tileUrl(props.tableId)}`],
    ...(bounds ? { bounds } : {}),
  });

  const { types } = await getGeometryTypes(props.tableId).catch(() => ({ types: [] as string[] }));
  const { hasPolygon, hasLine, hasPoint } = classifyGeometryTypes(types);

  if (hasPolygon) {
    m.addLayer({
      id: `${sourceId}-fill`,
      type: "fill",
      source: sourceId,
      "source-layer": "layer",
      paint: { "fill-color": "#6366f1", "fill-opacity": 0.35 },
    });
  }
  if (hasPolygon || hasLine) {
    m.addLayer({
      id: `${sourceId}-line`,
      type: "line",
      source: sourceId,
      "source-layer": "layer",
      paint: { "line-color": "#4338ca", "line-width": 1.5 },
    });
  }
  if (hasPoint) {
    m.addLayer({
      id: `${sourceId}-point`,
      type: "circle",
      source: sourceId,
      "source-layer": "layer",
      paint: { "circle-color": "#4338ca", "circle-radius": 4, "circle-stroke-color": "#ffffff", "circle-stroke-width": 1 },
    });
  }
}

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
    addTableLayer(map);

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
