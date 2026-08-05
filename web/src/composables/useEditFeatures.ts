import { ref, type Ref } from "vue";
import { getItems } from "../client/features";
import { errorMessage } from "../client/http";
import type { EditableFeature } from "./useEditLayer";
import { useToast } from "./useToast";

// How many rows a session may hold at once — over the **session**, not over
// one request: editing a boundary means panning along it, and a cap counted
// per view would let a dozen pans pile up a dozen capfuls. Editing is a hand
// operation on a neighbourhood, not a bulk one, so past this the honest
// answer is to carry what fits and say so rather than to freeze the browser
// (§8.6).
export const MAX_EDITABLE = 2000;

// A load that never answers must not leave the bar saying "Chargement…" for
// the rest of the session: past this the request is dropped and the failure
// is stated, which is the difference between a slow map and a dead one
// (§8.2).
const LOAD_TIMEOUT_MS = 30_000;

// What an edit session has on the map: the rows of the views it has visited,
// at full resolution — never a tile's simplified copy (§8.8) — under one cap
// counted over the whole session.
//
// It only ever hands back what the layer does not already hold: the map keeps
// geometries the user may have moved, and the server's copy of one of them
// must never come back over it.
export function useEditFeatures(tableId: Ref<string>, active: Ref<boolean>) {
  const toast = useToast();
  const loading = ref(false);
  const features = ref<EditableFeature[]>([]);
  // Set when the session could not carry everything it was asked for. Once
  // true it stays true for the session: what the map holds is partial from
  // then on, and a message that came and went as the user panned would read
  // as a glitch rather than as the statement it is.
  const truncated = ref(false);
  // Whether a load has come back at all. An empty layer and a layer not
  // answered for look the same on screen, and the bar must not tell the user
  // to drag vertices that are not there.
  const answered = ref(false);
  // The pkeys the session already holds. The map never gives them back, so
  // the count the cap is measured against is kept here, next to the requests
  // it bounds.
  const held = new Set<string>();
  // How many the session holds in total — not how many the last load added.
  // `features` carries only what was new, since handing back a row the map
  // already has would overwrite a geometry the user moved; so it is empty on
  // every pan over ground already covered, and reading emptiness off it told
  // the user there was nothing to edit while the map was full of it.
  const heldCount = ref(0);
  // A map move can outrun the load it triggered: only the newest answer may
  // land, or a pan that revealed rows would be undone by a slower, earlier
  // one and nothing would ask again until the user moved once more (§9).
  let latestRequest = 0;
  // The request in flight, so a superseded one is actually dropped rather
  // than left to run and land late.
  let inFlight: AbortController | null = null;

  function abortInFlight() {
    inFlight?.abort();
    inFlight = null;
  }

  function reset() {
    abortInFlight();
    features.value = [];
    truncated.value = false;
    answered.value = false;
    held.clear();
    heldCount.value = 0;
    latestRequest += 1;
    loading.value = false;
  }

  // Best-effort per pan: a view that fails to load leaves the session with
  // what it already holds rather than closing it under the user's hands.
  async function loadIn(bbox: [number, number, number, number]) {
    if (!active.value) return;
    const room = MAX_EDITABLE - held.size;
    if (room <= 0) {
      truncated.value = true;
      return;
    }

    abortInFlight();
    const controller = new AbortController();
    inFlight = controller;
    const timeout = setTimeout(() => controller.abort(), LOAD_TIMEOUT_MS);
    const request = ++latestRequest;
    loading.value = true;
    try {
      const collection = await getItems(tableId.value, { bbox, limit: room }, controller.signal);
      if (request !== latestRequest) return;
      if (collection.numberMatched > collection.numberReturned) truncated.value = true;
      const fresh = collection.features.flatMap((f) =>
        f.geometry && !held.has(f.id) ? [{ pkey: f.id, geometry: f.geometry }] : [],
      );
      for (const feature of fresh) held.add(feature.pkey);
      heldCount.value = held.size;
      features.value = fresh;
      answered.value = true;
    } catch (e) {
      // A request this composable dropped on purpose — a pan superseded it,
      // or the session closed — is not a failure to report. A timeout is.
      if (request === latestRequest) toast.error(errorMessage(e, "Impossible de charger les géométries à modifier"));
    } finally {
      clearTimeout(timeout);
      if (inFlight === controller) inFlight = null;
      // The newest request owns the flag, and it always releases it: a load
      // that failed, timed out or came back empty must never leave the bar
      // saying it is still loading.
      if (request === latestRequest) loading.value = false;
    }
  }

  return { features, loading, truncated, answered, heldCount, loadIn, reset };
}
