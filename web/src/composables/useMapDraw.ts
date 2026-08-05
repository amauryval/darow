import { onUnmounted, watch, type Ref } from "vue";
import type OlMap from "ol/Map";
import type MapBrowserEvent from "ol/MapBrowserEvent";
import type Feature from "ol/Feature";
import type Geometry from "ol/geom/Geometry";
import Modify, { type ModifyEvent } from "ol/interaction/Modify";
import Translate, { type TranslateEvent } from "ol/interaction/Translate";
import type { Condition } from "ol/events/condition";
import type { Coordinate } from "ol/coordinate";
import type { Pixel } from "ol/pixel";
import type { VectorSourceEvent } from "ol/source/Vector";
import { coordinateKey, handleStyle, verticesOf } from "../utils/drawStyles";
import { useEditLayer, type EditableFeature, type LayerChanges } from "./useEditLayer";
import { useEditTools } from "./useEditTools";

export type { DrawShape } from "./useEditTools";

// Right-click removes a vertex (§8.8). The engine deletes on alt+click by
// default, a keyboard modifier on a gesture that has a dedicated button.
const rightClick: Condition = (event) =>
  event.type === "pointerdown" && event.originalEvent instanceof PointerEvent && event.originalEvent.button === 2;

// The edit session's lifecycle: the layer it works on, the single interaction
// over it, and the teardown. Which tool the bar has armed is useEditTools'.
//
// Editing is a mode of the whole layer: every row of it is a candidate and
// they all land in one source, so a limit two polygons share moves on both
// sides in one gesture. Nothing is written here — the caller decides when the
// changes are sent, so a drag is never a submission (§8.2).

// How a session finds the geometries it does not hold yet. Nothing is loaded
// on entering the mode: the pointer says what is to be edited, and only that
// is fetched at full resolution (§8.8).
export interface MapDrawOptions {
  // Which row of the rendering underneath lies under a pixel — the layer the
  // user is reading, whichever render mode is on.
  pkeyAt: (pixel: Pixel) => string | null;
  // Asks for that row's geometry. Called on every frame the pointer rests
  // over a row the session does not hold, so whoever answers it owns the
  // de-duplication (useEditFeatures).
  reach: (pkey: string) => void;
}

export function useMapDraw(map: Ref<OlMap | null>, options: MapDrawOptions) {
  const layer = useEditLayer(map);
  const tools = useEditTools(map, layer.source);

  // Whether the session is armed at all. Not read off an interaction: which
  // one is on the map depends on the tool, and there are frames with none.
  let armed = false;
  let modify: Modify | null = null;
  let translate: Translate | null = null;
  let suppressMenu: ((event: MouseEvent) => void) | null = null;
  let onHover: ((event: MapBrowserEvent) => void) | null = null;
  let onLeave: (() => void) | null = null;
  // The frame the next hit test is scheduled on, and the pixel it will use.
  let hitScheduled: number | null = null;
  let lastPixel: Pixel | null = null;

  function onChange(event: VectorSourceEvent) {
    if (event.feature) layer.noteChange(event.feature);
  }

  // Which vertices a reshape moved. The engine says which *features* a
  // modification touched and never which vertex, so the geometry is
  // snapshotted when the drag starts and diffed when it ends: a coordinate
  // the geometry did not hold before is one the drag put there — a vertex
  // moved, or one inserted on a segment. A vertex deleted leaves no new
  // coordinate and nothing is marked; the feature's own outline already says
  // it changed.
  // Weakly keyed on purpose: it is dropped whole between drags, and it must
  // never be what keeps a released feature alive.
  let beforeModify = new WeakMap<Feature<Geometry>, Map<string, Coordinate>>();

  function coordinatesOf(feature: Feature<Geometry>): Coordinate[] {
    return verticesOf(feature.getGeometry())?.getCoordinates() ?? [];
  }

  function onModifyStart(event: ModifyEvent) {
    beforeModify = new WeakMap();
    event.features.forEach((feature) => {
      const before = new Map<string, Coordinate>();
      for (const coordinate of coordinatesOf(feature)) before.set(coordinateKey(coordinate), coordinate);
      beforeModify.set(feature, before);
    });
  }

  // Both halves of the diff, not just the new coordinates: the ones the drag
  // took away are where the moved nodes started, and that is what the leader
  // is drawn from.
  function onModifyEnd(event: ModifyEvent) {
    event.features.forEach((feature) => {
      const before = beforeModify.get(feature);
      if (!before) return;
      const after = new Map<string, Coordinate>();
      for (const coordinate of coordinatesOf(feature)) after.set(coordinateKey(coordinate), coordinate);
      const arrived = [...after].filter(([id]) => !before.has(id)).map(([, coordinate]) => coordinate);
      const left = [...before].filter(([id]) => !after.has(id)).map(([, coordinate]) => coordinate);
      layer.noteMoved(feature, arrived, left);
    });
    beforeModify = new WeakMap();
  }

  // A whole-feature move marks no vertex — every one of them moved, and
  // drawing them all is the cost this session is built to avoid — but it does
  // carry every mark already on the feature away from the geometry. Handing
  // noteMoved nothing is what makes it drop them.
  function onTranslateEnd(event: TranslateEvent) {
    event.features.forEach((feature) => layer.noteMoved(feature, [], []));
  }

  // Which drag the map answers: reshaping a geometry, or moving one whole.
  // One or the other, never both — see useEditTools.moving.
  //
  // What the Modify may touch is the whole of the topology switch. Over the
  // *source*, two geometries holding the same coordinate hold the same
  // vertex, so one drag moves both sides of a limit at once. Over a
  // *collection* holding only the feature under the pointer, the neighbour is
  // not in the interaction at all and its copy of the vertex stays put.
  //
  // Not `sharedVerticesEqual`: that comparison also tells Modify the pointer
  // is on a vertex rather than on a segment, so defeating it makes every
  // press insert a new vertex beside the old one instead of moving it.
  function retool(m: OlMap) {
    if (modify) m.removeInteraction(modify);
    if (translate) m.removeInteraction(translate);
    modify = null;
    translate = null;

    if (tools.moving.value) {
      // Over the hovered feature alone: a Translate handed the whole source
      // would drag every geometry the session holds in one gesture.
      translate = new Translate({ features: layer.solo });
      translate.on("translateend", onTranslateEnd);
      tools.addUnderSnap(m, translate);
      return;
    }

    modify = new Modify({
      ...(tools.topological.value ? { source: layer.source } : { features: layer.solo }),
      style: handleStyle,
      deleteCondition: rightClick,
    });
    modify.on("modifystart", onModifyStart);
    modify.on("modifyend", onModifyEnd);
    // Below the Snap, which has to see the pointer first to correct it.
    tools.addUnderSnap(m, modify);
  }

  function activate() {
    const m = map.value;
    if (!m || armed) return;
    armed = true;

    layer.add();
    // The Snap goes on before anything it corrects, and stays on for the
    // whole session: it is what the interaction order is built around.
    tools.arm();
    retool(m);
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
        if (!lastPixel) return;
        // What the session already holds is what it draws; anything else the
        // pointer rests over is a row of the rendering underneath, and
        // pointing at it is what brings it into the session.
        if (layer.trackPointer(m, lastPixel)) return;
        const pkey = options.pkeyAt(lastPixel);
        if (pkey !== null) options.reach(pkey);
      });
    };
    m.on("pointermove", onHover);

    // A pointer that left the map is over nothing: the vertices it was
    // showing have to go with it, or they stay lit on a feature no gesture
    // can reach any more.
    onLeave = () => layer.blur();
    m.getViewport().addEventListener("pointerleave", onLeave);
  }

  watch([tools.topological, tools.moving], () => {
    const m = map.value;
    if (m && armed) retool(m);
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
    armed = false;
    beforeModify = new WeakMap();
    layer.source.un("changefeature", onChange);
    layer.source.un("addfeature", onChange);
    tools.teardown();
    if (hitScheduled !== null) cancelAnimationFrame(hitScheduled);
    if (m) {
      if (modify) m.removeInteraction(modify);
      if (translate) m.removeInteraction(translate);
      if (onHover) m.un("pointermove", onHover);
      if (onLeave) m.getViewport().removeEventListener("pointerleave", onLeave);
      if (suppressMenu) m.getViewport().removeEventListener("contextmenu", suppressMenu);
    }
    modify = null;
    translate = null;
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
    moving: tools.moving,
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
    toggleMoving: tools.toggleMoving,
    toggleSnapping: tools.toggleSnapping,
    toggleTopology: tools.toggleTopology,
    undo: tools.undo,
  };
}
