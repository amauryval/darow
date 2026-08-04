import { onUnmounted, ref, shallowRef, type Ref } from "vue";
import { distance } from "@turf/distance";
import type { Map, MapMouseEvent } from "maplibre-gl";
import { handleFeatures } from "../utils/handles";
import { snapPosition } from "../utils/snapping";
import { draftGeometry, draftPositions, insertVertex, moveVertex, removeVertex, type DrawShape } from "../utils/vertices";
import { useDrawLayers } from "./useDrawLayers";
import { useVertexDrag } from "./useVertexDrag";

// A neighbour handed to the map: the row it belongs to, and its geometry as
// the server holds it.
export interface ReferenceFeature {
  pkey: string;
  geometry: GeoJSON.Geometry;
}

// How close to a neighbour a placed vertex has to land to be pulled onto
// it, in screen pixels — a tolerance in meters would mean one thing at the
// parcel's zoom and another at the district's.
const SNAP_PIXELS = 12;

// The map editor: MapLibre carries the pointer and the rendering, turf
// carries the geometry, and this holds the session — what is being drawn,
// where its vertices are, what it may snap to. Nothing is written here: the
// caller decides what a drawn geometry means and when it is saved, so a
// drag is never a submission (§8.2).
export function useMapDraw(map: Ref<Map | null>) {
  const pendingGeometry = shallowRef<GeoJSON.Geometry | null>(null);
  const snapping = ref(true);
  const shape = ref<DrawShape>("Polygon");
  const layers = useDrawLayers(map);

  // A session is either correcting an existing geometry — vertices are
  // dragged, never appended — or collecting the clicks of a new one.
  let correcting = false;
  let clicked: GeoJSON.Position[] = [];
  let references: GeoJSON.Geometry[] = [];
  let onClick: ((event: MapMouseEvent) => void) | null = null;

  function render() {
    const geometry = pendingGeometry.value;
    // Before a shape is one — a single click of a polygon-to-be — the
    // clicks are still drawn, as points: the user must see where they
    // landed, especially when snapping moved them.
    const provisional: GeoJSON.Geometry | null =
      geometry ?? (clicked.length > 0 ? { type: "MultiPoint", coordinates: clicked } : null);
    layers.renderShape(provisional);
    layers.renderHandles(handleFeatures(geometry));
  }

  function setGeometry(geometry: GeoJSON.Geometry | null) {
    pendingGeometry.value = geometry;
    // While drawing, the vertex list is the source of the next click's
    // shape: a vertex dragged or removed has to land back in it.
    if (!correcting && geometry) clicked = draftPositions(geometry);
    render();
  }

  // The pixel tolerance in meters at the current zoom, measured on the map
  // itself rather than assumed from the latitude.
  function tolerance(m: Map): number {
    const center = m.getCenter();
    const offset = m.unproject([m.project(center).x + SNAP_PIXELS, m.project(center).y]);
    return distance([center.lng, center.lat], [offset.lng, offset.lat], { units: "meters" });
  }

  function place(m: Map, position: GeoJSON.Position): GeoJSON.Position {
    return snapping.value ? snapPosition(position, references, tolerance(m)) : position;
  }

  const drag = useVertexDrag(map, {
    grab: (role, ref, position) => {
      const geometry = pendingGeometry.value;
      if (!geometry) return null;
      // A midpoint becomes the vertex it is dragged into: it is inserted
      // where it sat, and the address handed back is that new vertex's.
      if (role === "midpoint") setGeometry(insertVertex(geometry, ref, position));
      return ref;
    },
    move: (ref, position) => {
      const m = map.value;
      const geometry = pendingGeometry.value;
      if (!m || !geometry) return;
      setGeometry(moveVertex(geometry, ref, place(m, position)));
    },
    drop: () => render(),
    remove: (ref) => {
      const geometry = pendingGeometry.value;
      if (geometry) setGeometry(removeVertex(geometry, ref));
    },
  });

  function activate(initial: GeoJSON.Geometry | null) {
    const m = map.value;
    if (!m || onClick) return;

    correcting = initial !== null;
    clicked = [];
    layers.add();
    setGeometry(initial);

    // A double-click while drawing is two vertices, never a zoom.
    m.doubleClickZoom.disable();
    onClick = (event) => {
      if (correcting) return;
      const position = place(m, [event.lngLat.lng, event.lngLat.lat]);
      clicked = shape.value === "Point" ? [position] : [...clicked, position];
      setGeometry(draftGeometry(shape.value, clicked));
    };
    m.on("click", onClick);
    drag.activate();
  }

  // Snapping is what makes two features share an exact boundary, and what
  // gets in the way when they must not — hence a toggle rather than a
  // setting.
  function toggleSnapping() {
    snapping.value = !snapping.value;
  }

  // Changing the shape restarts the drawing: three clicks meant as a
  // polygon are not the same three clicks meant as a line.
  function setShape(next: DrawShape) {
    if (correcting || next === shape.value) return;
    shape.value = next;
    clicked = [];
    setGeometry(null);
  }

  // Takes back the last click, so a misplaced vertex does not mean
  // restarting the shape.
  function undo() {
    if (correcting || clicked.length === 0) return;
    clicked = clicked.slice(0, -1);
    setGeometry(draftGeometry(shape.value, clicked));
  }

  // Snapping only ever sees what it is given, so the neighbours loaded
  // around the edited shape are handed here — and drawn, as outlines, so
  // the user aims at something visible.
  function setReference(features: ReferenceFeature[]) {
    references = features.map((f) => f.geometry);
    layers.renderReference(references);
  }

  function deactivate() {
    const m = map.value;
    drag.deactivate();
    if (m && onClick) {
      m.off("click", onClick);
      m.doubleClickZoom.enable();
    }
    onClick = null;
    references = [];
    clicked = [];
    pendingGeometry.value = null;
    layers.remove();
  }

  onUnmounted(() => {
    deactivate();
  });

  return { pendingGeometry, snapping, shape, activate, deactivate, setReference, setShape, toggleSnapping, undo };
}
