import { onUnmounted, watch, type Ref } from "vue";
import type Map from "ol/Map";
import type MapBrowserEvent from "ol/MapBrowserEvent";
import Modify from "ol/interaction/Modify";
import type { Condition } from "ol/events/condition";
import type { Pixel } from "ol/pixel";
import type { VectorSourceEvent } from "ol/source/Vector";
import { handleStyle } from "../utils/drawStyles";
import { useEditLayer, type EditableFeature, type LayerChanges } from "./useEditLayer";
import { useEditTools } from "./useEditTools";

export type { DrawShape } from "./useEditTools";

// Right-click removes a vertex (§8.8). The engine deletes on alt+click by
// default, a keyboard modifier on a gesture that has a dedicated button.
const rightClick: Condition = (event) =>
  event.type === "pointerdown" && event.originalEvent instanceof PointerEvent && event.originalEvent.button === 2;

// The edit session's lifecycle: the layer it works on, the one Modify over
// it, and the teardown. Which tool the bar has armed is useEditTools'.
//
// Editing is a mode of the whole layer: every feature in view is editable, so
// a limit two polygons share moves on both sides in one gesture. Nothing is
// written here — the caller decides when the changes are sent, so a drag is
// never a submission (§8.2).
export function useMapDraw(map: Ref<Map | null>) {
  const layer = useEditLayer(map);
  const tools = useEditTools(map, layer.source);

  let modify: Modify | null = null;
  let suppressMenu: ((event: MouseEvent) => void) | null = null;
  let onHover: ((event: MapBrowserEvent) => void) | null = null;
  let onLeave: (() => void) | null = null;
  // The frame the next hit test is scheduled on, and the pixel it will use.
  let hitScheduled: number | null = null;
  let lastPixel: Pixel | null = null;

  function onChange(event: VectorSourceEvent) {
    if (event.feature) layer.noteChange(event.feature);
  }

  // What the Modify may touch — the whole of the topology switch. Over the
  // *source*, two geometries holding the same coordinate hold the same
  // vertex, so one drag moves both sides of a limit at once. Over a
  // *collection* holding only the feature under the pointer, the neighbour is
  // not in the interaction at all and its copy of the vertex stays put.
  //
  // Not `sharedVerticesEqual`: that comparison also tells Modify the pointer
  // is on a vertex rather than on a segment, so defeating it makes every
  // press insert a new vertex beside the old one instead of moving it.
  function remodify(m: Map) {
    if (modify) m.removeInteraction(modify);
    modify = new Modify({
      ...(tools.topological.value ? { source: layer.source } : { features: layer.solo }),
      style: handleStyle,
      deleteCondition: rightClick,
    });
    // Below the Snap, which has to see the pointer first to correct it.
    tools.addUnderSnap(m, modify);
  }

  function activate() {
    const m = map.value;
    if (!m || modify) return;

    layer.add();
    // The Snap goes on before anything it corrects, and stays on for the
    // whole session: it is what the interaction order is built around.
    tools.arm();
    remodify(m);
    layer.source.on("changefeature", onChange);
    layer.source.on("addfeature", onChange);

    // A right-click is the gesture that removes a vertex, so it must not also
    // open the browser's menu over the map.
    suppressMenu = (event) => event.preventDefault();
    m.getViewport().addEventListener("contextmenu", suppressMenu);

    // Which feature the pointer rests over is now needed whatever the
    // topology switch says: with topology off it is what the Modify follows,
    // and either way it is the one geometry whose vertices are drawn.
    //
    // Hit detection over a whole view is not free, and a pointer emits far
    // more moves than the screen has frames — so at most one test per frame,
    // on the last pixel seen. Nothing is tracked mid-drag: the interaction
    // has already chosen its feature, and re-testing would only fight it.
    onHover = (event) => {
      if (event.dragging) return;
      lastPixel = event.pixel;
      if (hitScheduled !== null) return;
      hitScheduled = requestAnimationFrame(() => {
        hitScheduled = null;
        if (lastPixel) layer.trackPointer(m, lastPixel);
      });
    };
    m.on("pointermove", onHover);

    // A pointer that left the map is over nothing: the vertices it was
    // showing have to go with it, or they stay lit on a feature no gesture
    // can reach any more.
    onLeave = () => layer.blur();
    m.getViewport().addEventListener("pointerleave", onLeave);
  }

  watch(tools.topological, () => {
    const m = map.value;
    if (m && modify) remodify(m);
  });

  // What the session would write, dropping whatever was half-drawn.
  function commit(): LayerChanges {
    tools.abort();
    return layer.changes();
  }

  // The rows the view has gained, at full resolution. The caller owns the
  // network; what the layer already holds is left alone (useEditLayer.load).
  function load(features: EditableFeature[]) {
    layer.load(features);
  }

  // The rows the session may stop holding, so its budget is not a one-way
  // ratchet. The caller reports them to whoever counts them.
  function prune(): string[] {
    return layer.prune();
  }

  function deactivate() {
    const m = map.value;
    layer.source.un("changefeature", onChange);
    layer.source.un("addfeature", onChange);
    tools.teardown();
    if (hitScheduled !== null) cancelAnimationFrame(hitScheduled);
    if (m) {
      if (modify) m.removeInteraction(modify);
      if (onHover) m.un("pointermove", onHover);
      if (onLeave) m.getViewport().removeEventListener("pointerleave", onLeave);
      if (suppressMenu) m.getViewport().removeEventListener("contextmenu", suppressMenu);
    }
    modify = null;
    onHover = null;
    onLeave = null;
    suppressMenu = null;
    hitScheduled = null;
    lastPixel = null;
    layer.remove();
  }

  onUnmounted(deactivate);

  return {
    shape: tools.shape,
    snapping: tools.snapping,
    topological: tools.topological,
    cuttingHole: tools.cuttingHole,
    drawing: tools.drawing,
    dirtyCount: layer.dirtyCount,
    changedCount: layer.changedCount,
    createdCount: layer.createdCount,
    hasAreal: layer.hasAreal,
    activate,
    deactivate,
    commit,
    load,
    prune,
    setShape: tools.setShape,
    toggleHole: tools.toggleHole,
    toggleSnapping: tools.toggleSnapping,
    toggleTopology: tools.toggleTopology,
    undo: tools.undo,
  };
}
