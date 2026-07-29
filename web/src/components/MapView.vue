<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import maplibregl, { Map, Popup } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { getGeometryTypes, tileUrl } from "../client/geo";
import { getRow } from "../client/tables";
import { errorMessage } from "../client/http";

const props = defineProps<{
  tableId: string;
  bbox?: string;
}>();

const container = ref<HTMLDivElement | null>(null);
let map: Map | null = null;
let popup: Popup | null = null;

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

// Only fill/polygon layers register a dedicated hit target for click —
// the "-line" outline stroke is a few px wide and would make clicking a
// polygon's interior miss, so inspection always targets fill/circle first.
async function addTableLayer(m: Map): Promise<string[]> {
  const sourceId = `table-${props.tableId}`;
  const bounds = props.bbox ? parseBbox(props.bbox) : null;
  m.addSource(sourceId, {
    type: "vector",
    tiles: [`${location.origin}${tileUrl(props.tableId)}`],
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
      "source-layer": "layer",
      paint: { "fill-color": "#6366f1", "fill-opacity": 0.35 },
    });
    interactiveLayerIds.push(id);
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
    const id = `${sourceId}-point`;
    m.addLayer({
      id,
      type: "circle",
      source: sourceId,
      "source-layer": "layer",
      paint: { "circle-color": "#4338ca", "circle-radius": 4, "circle-stroke-color": "#ffffff", "circle-stroke-width": 1 },
    });
    interactiveLayerIds.push(id);
  }
  return interactiveLayerIds;
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
}

// The MVT tile only embeds the pkey attribute (règle 7 excludes the
// geometry column, and no other attrs are requested here) — the rest of
// the row's data is joined client-side by fetching it over REST once a
// feature is clicked, instead of bloating every tile with every column.
async function inspectFeature(m: Map, point: maplibregl.PointLike, lngLat: maplibregl.LngLatLike, layerIds: string[]) {
  const feature = m.queryRenderedFeatures(point, { layers: layerIds })[0];
  const pkey = feature?.properties?.pkey;
  if (pkey === undefined || pkey === null) return;

  popup?.remove();
  popup = new maplibregl.Popup({ closeButton: true, maxWidth: "280px" })
    .setLngLat(lngLat)
    .setHTML('<div style="font-size:12px;color:#64748b;">Chargement…</div>')
    .addTo(m);

  try {
    const { row } = await getRow(props.tableId, String(pkey));
    const entries = Object.entries(row).filter(([k]) => k !== "pkey");
    const body = entries
      .map(
        ([k, v]) =>
          `<tr><td style="padding:2px 8px 2px 0;color:#64748b;font-weight:500;white-space:nowrap;">${escapeHtml(k)}</td>` +
          `<td style="padding:2px 0;color:#0f172a;">${escapeHtml(v === null ? "—" : String(v))}</td></tr>`,
      )
      .join("");
    popup.setHTML(
      `<table style="font-size:12px;border-collapse:collapse;"><tbody>${body || "<tr><td>Aucun attribut.</td></tr>"}</tbody></table>`,
    );
  } catch (e) {
    popup.setHTML(`<div style="font-size:12px;color:#dc2626;">${escapeHtml(errorMessage(e, "Impossible de charger la ligne"))}</div>`);
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
    addTableLayer(map).then((layerIds) => {
      if (!map || layerIds.length === 0) return;
      const m = map;
      m.on("mouseenter", layerIds, () => (m.getCanvas().style.cursor = "pointer"));
      m.on("mouseleave", layerIds, () => (m.getCanvas().style.cursor = ""));
      m.on("click", layerIds, (e) => inspectFeature(m, e.point, e.lngLat, layerIds));
    });

    const bounds = props.bbox ? parseBbox(props.bbox) : null;
    if (bounds) {
      map.fitBounds(bounds, { padding: 32, animate: false });
    }
  });
});

onUnmounted(() => {
  popup?.remove();
  popup = null;
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
