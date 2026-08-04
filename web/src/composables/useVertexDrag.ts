import type { Map, MapMouseEvent } from "maplibre-gl";
import type { Ref } from "vue";
import { HANDLES } from "./useDrawLayers";
import { handleRef, type HandleRole } from "../utils/handles";
import type { VertexRef } from "../utils/vertices";

// The pointer half of the editor: grabbing a handle, dragging it, dropping
// it, and right-clicking a vertex away. It decides nothing about the
// geometry — it reports the address it grabbed and where the pointer is,
// and the caller answers what that means.
//
// Hit-testing goes through the handle layers rather than DOM markers: one
// source, one query, and a parcel with three hundred vertices costs the
// same as one with four.
export interface VertexDragHandlers {
  // Answers the address to drag from a grabbed handle and the position it
  // sits at — a midpoint becomes a real vertex here — or null to let the
  // gesture through to the map.
  grab: (role: HandleRole, ref: VertexRef, position: GeoJSON.Position) => VertexRef | null;
  move: (ref: VertexRef, position: GeoJSON.Position) => void;
  drop: () => void;
  remove: (ref: VertexRef) => void;
}

const LAYERS = [`${HANDLES}-vertex`, `${HANDLES}-midpoint`];

export function useVertexDrag(map: Ref<Map | null>, handlers: VertexDragHandlers) {
  let dragging: VertexRef | null = null;
  let bound: Array<() => void> = [];

  function grabbed(event: MapMouseEvent) {
    const m = map.value;
    if (!m) return;
    const feature = m.queryRenderedFeatures(event.point, { layers: LAYERS.filter((id) => m.getLayer(id)) })[0];
    const handle = feature ? handleRef(feature.properties) : null;
    if (!handle || feature.geometry.type !== "Point") return;

    const ref = handlers.grab(handle.role, handle.ref, feature.geometry.coordinates);
    if (!ref) return;
    dragging = ref;
    // A drag on a vertex must not also drag the map under it.
    event.preventDefault();
    m.dragPan.disable();
    m.getCanvas().style.cursor = "grabbing";
  }

  function moved(event: MapMouseEvent) {
    if (!dragging) return;
    handlers.move(dragging, [event.lngLat.lng, event.lngLat.lat]);
  }

  function dropped() {
    if (!dragging) return;
    dragging = null;
    map.value?.dragPan.enable();
    if (map.value) map.value.getCanvas().style.cursor = "";
    handlers.drop();
  }

  function removed(event: MapMouseEvent) {
    const m = map.value;
    if (!m) return;
    const layer = `${HANDLES}-vertex`;
    if (!m.getLayer(layer)) return;
    const feature = m.queryRenderedFeatures(event.point, { layers: [layer] })[0];
    const handle = feature ? handleRef(feature.properties) : null;
    if (!handle || handle.role !== "vertex") return;
    event.preventDefault();
    handlers.remove(handle.ref);
  }

  function enter() {
    if (map.value && !dragging) map.value.getCanvas().style.cursor = "move";
  }

  function leave() {
    if (map.value && !dragging) map.value.getCanvas().style.cursor = "";
  }

  function activate() {
    const m = map.value;
    if (!m || bound.length > 0) return;

    m.on("mousedown", grabbed);
    m.on("mousemove", moved);
    m.on("mouseup", dropped);
    m.on("contextmenu", removed);
    bound = [
      () => m.off("mousedown", grabbed),
      () => m.off("mousemove", moved),
      () => m.off("mouseup", dropped),
      () => m.off("contextmenu", removed),
    ];
    for (const layer of LAYERS) {
      if (!m.getLayer(layer)) continue;
      m.on("mouseenter", layer, enter);
      m.on("mouseleave", layer, leave);
      bound.push(
        () => m.off("mouseenter", layer, enter),
        () => m.off("mouseleave", layer, leave),
      );
    }
  }

  function deactivate() {
    dropped();
    for (const off of bound) off();
    bound = [];
  }

  return { activate, deactivate };
}
