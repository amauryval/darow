import { watch, type Ref } from "vue";
import { shiftKeyOnly } from "ol/events/condition";
import type Map from "ol/Map";
import type Layer from "ol/layer/Layer";
import type MapBrowserEvent from "ol/MapBrowserEvent";
import type { Pixel } from "ol/pixel";

export interface FeatureSelection {
  selectedPkeys: Ref<string[]>;
  toggle: (pkey: string) => void;
  // Whether a **plain** click means "select". It does not during an edit
  // session: the same click grabs a handle, and the engine inserts a vertex
  // on pointerdown over a segment — so the session asks for shift instead,
  // and selecting stays possible while editing (§8.8). The hover cursor
  // follows the same rule: it would fight the one Modify sets.
  selectable: Ref<boolean>;
}

// The half of a render mode that has nothing to do with how the table is
// fetched: which features are selected, and what a click on one means. Both
// modes wire it the same way, so they share it rather than each keeping
// their own copy of the same three listeners.
//
// The selection itself lives in useFeatureSelection and is never held here:
// this reflects it onto the layer and reports clicks back through toggle().
// OpenLayers has no per-feature state — a style function reads the set on
// every frame instead, which is why a change has to tell the layer to
// repaint.
// repaint is how a layer is told the selection moved. The two render modes
// answer it differently — a canvas layer re-runs its style function, a WebGL
// one is handed a new style — so the layer decides, not this.
export function useMapSelection(map: Ref<Map | null>, selection: FeatureSelection, repaint: (pkeys: string[]) => void) {
  let selected = new Set(selection.selectedPkeys.value);
  let layer: Layer | null = null;
  let onClick: ((event: MapBrowserEvent) => void) | null = null;
  let onMove: ((event: MapBrowserEvent) => void) | null = null;

  // Passed to a layer's style function: the answer must not depend on
  // anything but the feature, since it is asked once per feature per frame.
  function isSelected(id: string | number | undefined): boolean {
    return id !== undefined && selected.has(String(id));
  }

  // Which row of this render mode's layer lies under a pixel. Selection is
  // one reader of it and an edit session is the other: pointing at a feature
  // is how its geometry is brought into the session (§8.8), and that happens
  // precisely while these listeners are unbound.
  function pkeyAt(pixel: Pixel): string | null {
    const m = map.value;
    if (!m || !layer) return null;
    const found = m.forEachFeatureAtPixel(pixel, (feature) => feature.getId(), {
      layerFilter: (candidate) => candidate === layer,
    });
    return found === undefined ? null : String(found);
  }

  function bind() {
    const m = map.value;
    if (!m || onClick) return;

    // One click toggles the clicked feature in and out of the selection, with
    // no modifier key — so the gesture stays usable on a trackpad (§8.8).
    // Under an edit session that same click belongs to the editor, and only
    // a shift-click still selects.
    onClick = (event) => {
      if (!selection.selectable.value && !shiftKeyOnly(event)) return;
      const id = pkeyAt(event.pixel);
      if (id !== null) selection.toggle(id);
    };
    onMove = (event) => {
      if (event.dragging || !selection.selectable.value) return;
      m.getTargetElement().style.cursor = pkeyAt(event.pixel) === null ? "" : "pointer";
    };
    m.on("singleclick", onClick);
    m.on("pointermove", onMove);
  }

  function unbind() {
    const m = map.value;
    if (m && onClick) m.un("singleclick", onClick);
    if (m && onMove) m.un("pointermove", onMove);
    // The cursor is the map's, not this layer's: whatever set it last has to
    // give it back, or an edit session inherits a pointer over every feature.
    if (m) m.getTargetElement().style.cursor = "";
    onClick = null;
    onMove = null;
  }

  function activate(target: Layer) {
    layer = target;
    bind();
  }

  function deactivate() {
    unbind();
    layer = null;
  }

  // The listeners stay bound for the whole life of the layer: what an edit
  // session suspends is the plain click and the hover cursor, not selecting
  // itself (the click handler reads `selectable` on every event). What still
  // has to happen on the switch is giving the cursor back — the session sets
  // its own, and an inherited pointer over every feature is not it.
  watch(selection.selectable, (on) => {
    const m = map.value;
    if (!on && m) m.getTargetElement().style.cursor = "";
  });

  watch(selection.selectedPkeys, (pkeys) => {
    selected = new Set(pkeys);
    if (layer) repaint(pkeys);
  });

  return { isSelected, pkeyAt, activate, deactivate };
}
