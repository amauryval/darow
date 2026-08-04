import type { Ref } from "vue";
import type { GeoJSONSource, Map } from "maplibre-gl";

// The three things an edit session draws on top of the table's own
// rendering: the neighbours it may snap to, the shape being entered, and
// the handles. Everything is a GeoJSON source — the editor keeps no state
// here, it pushes what it holds and this reads back nothing.
const REFERENCE = "draw-reference";
const SHAPE = "draw-shape";
export const HANDLES = "draw-handles";

const EMPTY: GeoJSON.FeatureCollection = { type: "FeatureCollection", features: [] };

function features(geometries: GeoJSON.Geometry[]): GeoJSON.FeatureCollection {
  return { type: "FeatureCollection", features: geometries.map((geometry) => ({ type: "Feature", geometry, properties: {} })) };
}

export function useDrawLayers(map: Ref<Map | null>) {
  const sourceIds = [REFERENCE, SHAPE, HANDLES];
  const layerIds = [
    `${REFERENCE}-line`,
    `${SHAPE}-fill`,
    `${SHAPE}-line`,
    `${SHAPE}-point`,
    `${HANDLES}-midpoint`,
    `${HANDLES}-vertex`,
  ];
  let added = false;

  function add() {
    const m = map.value;
    if (!m || added) return;
    added = true;

    for (const id of sourceIds) m.addSource(id, { type: "geojson", data: EMPTY });

    // The neighbours are drawn as bare outlines: they are there to snap to
    // and to read, never to be confused with what is being edited.
    m.addLayer({
      id: `${REFERENCE}-line`,
      type: "line",
      source: REFERENCE,
      paint: { "line-color": "#64748b", "line-width": 1.5, "line-dasharray": [2, 2] },
    });
    m.addLayer({
      id: `${SHAPE}-fill`,
      type: "fill",
      source: SHAPE,
      filter: ["==", ["geometry-type"], "Polygon"],
      paint: { "fill-color": "#2563eb", "fill-opacity": 0.2 },
    });
    m.addLayer({
      id: `${SHAPE}-line`,
      type: "line",
      source: SHAPE,
      paint: { "line-color": "#2563eb", "line-width": 2 },
    });
    // A point geometry, and the clicks of a shape that is not one yet: both
    // are what the user has placed so far and both must be visible.
    m.addLayer({
      id: `${SHAPE}-point`,
      type: "circle",
      source: SHAPE,
      filter: ["==", ["geometry-type"], "Point"],
      paint: { "circle-radius": 4, "circle-color": "#2563eb" },
    });
    // A midpoint is deliberately quieter than a vertex: same gesture, but
    // one moves what exists and the other creates.
    m.addLayer({
      id: `${HANDLES}-midpoint`,
      type: "circle",
      source: HANDLES,
      filter: ["==", ["get", "role"], "midpoint"],
      paint: {
        "circle-radius": 3.5,
        "circle-color": "#ffffff",
        "circle-opacity": 0.7,
        "circle-stroke-color": "#2563eb",
        "circle-stroke-width": 1,
      },
    });
    m.addLayer({
      id: `${HANDLES}-vertex`,
      type: "circle",
      source: HANDLES,
      filter: ["==", ["get", "role"], "vertex"],
      paint: {
        "circle-radius": 5,
        "circle-color": "#ffffff",
        "circle-stroke-color": "#2563eb",
        "circle-stroke-width": 2,
      },
    });
  }

  function setData(id: string, data: GeoJSON.FeatureCollection) {
    map.value?.getSource<GeoJSONSource>(id)?.setData(data);
  }

  function renderReference(geometries: GeoJSON.Geometry[]) {
    setData(REFERENCE, features(geometries));
  }

  function renderShape(geometry: GeoJSON.Geometry | null) {
    setData(SHAPE, features(geometry ? [geometry] : []));
  }

  function renderHandles(handles: GeoJSON.FeatureCollection) {
    setData(HANDLES, handles);
  }

  function remove() {
    const m = map.value;
    if (!m || !added) return;
    added = false;
    for (const id of layerIds) if (m.getLayer(id)) m.removeLayer(id);
    for (const id of sourceIds) if (m.getSource(id)) m.removeSource(id);
  }

  return { add, remove, renderReference, renderShape, renderHandles };
}
