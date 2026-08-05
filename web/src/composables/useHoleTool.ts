import { ref, type Ref } from "vue";
import type Map from "ol/Map";
import Draw from "ol/interaction/Draw";
import type Interaction from "ol/interaction/Interaction";
import Polygon from "ol/geom/Polygon";
import type VectorSource from "ol/source/Vector";
import { punchHole } from "../utils/holes";
import { useToast } from "./useToast";

// Cutting a hole is drawing a ring inside a shape and handing it to that
// shape's geometry rather than to the source: the ring becomes an interior
// ring of the polygon it was drawn in, and its vertices are then dragged,
// inserted and removed like any other (`utils/holes.ts`). Which is also how
// a hole is undone — there is no second, opposite tool to learn.
//
// Which polygon it belongs to is decided by where it was drawn, not by a
// prior selection: in a layer-wide edit session there is no chosen feature.
export interface HoleToolHooks {
  // How the ring's Draw joins the map: the session owns the interaction
  // order, and a Draw added over the Snap would never be snapped.
  attach: (map: Map, interaction: Interaction) => void;
  onCut: () => void;
}

export function useHoleTool(map: Ref<Map | null>, source: VectorSource, hooks: HoleToolHooks) {
  const toast = useToast();
  const cutting = ref(false);
  let draw: Draw | null = null;

  function stop() {
    const m = map.value;
    if (m && draw) m.removeInteraction(draw);
    draw = null;
    cutting.value = false;
  }

  function cut(ring: Polygon): boolean {
    for (const feature of source.getFeatures()) {
      const geometry = feature.getGeometry();
      const punched = geometry ? punchHole(geometry, ring) : null;
      if (punched) {
        feature.setGeometry(punched);
        return true;
      }
    }
    return false;
  }

  function start() {
    const m = map.value;
    if (!m) return;
    // No source: the drawn ring is never a feature of its own, it is only
    // ever the shape of a hole in another one.
    const started = new Draw({ type: "Polygon" });
    started.on("drawend", (event) => {
      const drawn = event.feature.getGeometry();
      // A ring that belongs to no polygon is dropped — it has to be said, or
      // the work disappears with nothing on screen accounting for it (§8.2).
      if (!(drawn instanceof Polygon) || !cut(drawn)) {
        toast.error("Ce trou n'est dans aucun polygone : dessinez-le entièrement à l'intérieur d'une forme.");
      }
      stop();
      hooks.onCut();
    });
    draw = started;
    cutting.value = true;
    hooks.attach(m, started);
  }

  function toggle() {
    if (draw) stop();
    else start();
  }

  function removeLastPoint() {
    draw?.removeLastPoint();
  }

  return { cutting, active: () => draw !== null, toggle, stop, removeLastPoint };
}
