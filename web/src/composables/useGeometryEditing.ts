import { computed, ref, type Ref } from "vue";
import { addRows, getRowGeometry, updateRows } from "../client/tables";
import { getNeighbours } from "../client/geo";
import { errorMessage } from "../client/http";
import { isEditableGeometry } from "../utils/geometry";
import { freedPieces, isAreal, reshapeNeighbour } from "../utils/topology";
import type { ReferenceFeature } from "./useMapDraw";
import { useToast } from "./useToast";

// What the map is being used for right now: nothing, placing the geometry
// of a row being created, or correcting an existing row's.
export type GeometryEditingMode = "idle" | "create" | "edit";

// How far around the edited shape a neighbour is looked for, in meters —
// the server answers with an ST_Intersects against a buffer of that radius,
// so an L-shaped parcel catches what it really touches and not what its
// bounding box happens to cover.
const NEIGHBOUR_BUFFER_METERS = 50;
const MAX_NEIGHBOURS = 200;

// Entering a geometry on the map, in the two shapes it takes: a new row,
// whose attributes are collected first and whose geometry is drawn second,
// and an existing row whose geometry is reshaped in place. Editing can be
// topological, and that is the drawing engine's business: the neighbours
// are loaded into it so its snapping and vertex-pinning modes have
// something to work against, and whichever of them the user moved is saved
// along. The write is server-authoritative — nothing is drawn as saved
// until the server answers (§8.2).
export function useGeometryEditing(tableId: Ref<string>, geometryColumn: Ref<string | undefined>, onMutated: () => void) {
  const toast = useToast();
  const mode = ref<GeometryEditingMode>("idle");
  const editingPkey = ref<string | null>(null);
  const initialGeometry = ref<GeoJSON.Geometry | null>(null);
  const neighbours = ref<ReferenceFeature[]>([]);
  const saving = ref(false);
  const loading = ref(false);
  const draftValues = ref<Record<string, unknown>>({});
  // Recutting the neighbours is the point of an edit on a coverage
  // (parcels, zonings) and a nuisance on shapes that merely touch — so it
  // is a mode the user turns off, not a behaviour of the editor. On by
  // default: an edit session is opened on adjacent data far more often
  // than not.
  const topology = ref(true);

  const active = computed(() => mode.value !== "idle");
  const prompt = computed(() =>
    mode.value === "create"
      ? "Cliquez pour poser les sommets, puis enregistrez."
      : topology.value
        ? "Déplacez les sommets, puis enregistrez. Les voisins sont recoupés sur la nouvelle limite."
        : "Déplacez les sommets, puis enregistrez. Les voisins ne sont pas modifiés.",
  );

  function reset() {
    mode.value = "idle";
    editingPkey.value = null;
    initialGeometry.value = null;
    neighbours.value = [];
    draftValues.value = {};
  }

  // Best-effort: without its neighbours an edit still works, it just loses
  // snapping and the shared-boundary follow-up, so a failure here is a
  // warning rather than a blocked session.
  async function loadNeighbours(geometry: GeoJSON.Geometry, exclude: string[]) {
    try {
      const collection = await getNeighbours(tableId.value, geometry, NEIGHBOUR_BUFFER_METERS, exclude, MAX_NEIGHBOURS);
      neighbours.value = collection.features.flatMap((f) => (f.geometry ? [{ pkey: f.id, geometry: f.geometry }] : []));
    } catch {
      neighbours.value = [];
      toast.error("Les géométries voisines n'ont pas pu être chargées : l'aimantation ne fonctionnera pas.");
    }
  }

  // The attributes are already in hand; only the geometry is missing, so
  // the row is posted once — a half-created row never reaches the table.
  function startCreate(values: Record<string, unknown>) {
    draftValues.value = values;
    editingPkey.value = null;
    initialGeometry.value = null;
    neighbours.value = [];
    mode.value = "create";
  }

  async function startEdit(pkey: string) {
    loading.value = true;
    try {
      const { geometry } = await getRowGeometry(tableId.value, pkey);
      if (geometry !== null && !isEditableGeometry(geometry)) {
        toast.error("Cette géométrie n'est pas modifiable sur la carte.");
        return;
      }
      editingPkey.value = pkey;
      initialGeometry.value = geometry;
      mode.value = "edit";
      if (geometry) await loadNeighbours(geometry, [pkey]);
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de charger la géométrie de cette ligne"));
    } finally {
      loading.value = false;
    }
  }

  // The neighbours to write, in topological mode: those the reshaped
  // geometry now covers lose that area, those the retreat freed gain the
  // pieces touching them. It is turf that recuts them — set operations on
  // the shapes, not vertex bookkeeping — so the result holds whatever the
  // user did to the boundary, vertices added or removed included.
  function affectedNeighbours(geometry: GeoJSON.Geometry) {
    const updates = new Map<string, GeoJSON.Geometry>();
    const before = initialGeometry.value;
    if (!topology.value || !before || !isAreal(before) || !isAreal(geometry)) return updates;

    const freed = freedPieces(before, geometry);
    for (const neighbour of neighbours.value) {
      if (!isAreal(neighbour.geometry)) continue;
      const reshaped = reshapeNeighbour(neighbour.geometry, geometry, freed);
      if (reshaped) updates.set(neighbour.pkey, reshaped);
    }
    return updates;
  }

  async function save(geometry: GeoJSON.Geometry) {
    const column = geometryColumn.value;
    if (!column) return;
    saving.value = true;
    try {
      const updates = mode.value === "edit" ? affectedNeighbours(geometry) : new Map<string, GeoJSON.Geometry>();

      if (mode.value === "create") {
        await addRows(tableId.value, [{ ...draftValues.value, [column]: geometry }]);
      } else if (editingPkey.value) {
        // The edited row and every neighbour it moved go in one request:
        // the server writes them in a single transaction, so a shared
        // boundary is never left moved on one side only.
        await updateRows(tableId.value, [
          { pkey: editingPkey.value, values: { [column]: geometry } },
          ...[...updates].map(([pkey, neighbourGeometry]) => ({ pkey, values: { [column]: neighbourGeometry } })),
        ]);
      }

      toast.success(
        updates.size > 0
          ? `Géométrie enregistrée, ${updates.size} voisine(s) ajustée(s).`
          : mode.value === "create"
            ? "Ligne ajoutée."
            : "Géométrie mise à jour.",
      );
      reset();
      onMutated();
    } catch (e) {
      toast.error(errorMessage(e, "Impossible d'enregistrer la géométrie"));
    } finally {
      saving.value = false;
    }
  }

  function cancel() {
    reset();
  }

  return {
    mode,
    active,
    prompt,
    editingPkey,
    initialGeometry,
    neighbours,
    topology,
    draftValues,
    saving,
    loading,
    startCreate,
    startEdit,
    save,
    cancel,
  };
}
