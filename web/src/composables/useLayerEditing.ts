import { computed, ref, watch, type Ref } from "vue";
import { addRows, updateRows } from "../client/tables";
import { errorMessage } from "../client/http";
import type { LayerChanges } from "./useEditLayer";
import { MAX_EDITABLE, useEditFeatures } from "./useEditFeatures";
import { editingPrompt, summarizeWrites } from "../utils/editPrompts";
import { useToast } from "./useToast";

// Editing a geo table's geometries, as a mode of the layer rather than of one
// row: what the mode is waiting for, and what leaving it writes. What the
// session carries on the map is useEditFeatures'.
//
// The write is server-authoritative: nothing is drawn as saved until the
// server answers, and every moved geometry goes in one request so a limit
// two rows share is never left moved on one side only (§8.2).
export function useLayerEditing(tableId: Ref<string>, geometryColumn: Ref<string | undefined>, onMutated: () => void) {
  const toast = useToast();
  const active = ref(false);
  const saving = ref(false);
  // A row created from the grid brings its attributes with it; its geometry
  // is the first shape drawn in the session that follows.
  const draftValues = ref<Record<string, unknown>>({});
  const loaded = useEditFeatures(tableId, active);

  const pendingRow = computed(() => Object.keys(draftValues.value).length > 0);

  const prompt = computed(() =>
    editingPrompt({
      loading: loaded.loading.value,
      truncated: loaded.truncated.value,
      pendingRow: pendingRow.value,
      empty: loaded.answered.value && loaded.heldCount.value === 0,
      max: MAX_EDITABLE,
    }),
  );

  function reset() {
    active.value = false;
    draftValues.value = {};
    loaded.reset();
  }

  function start(values: Record<string, unknown> = {}) {
    reset();
    draftValues.value = values;
    active.value = true;
  }

  async function save(changes: LayerChanges) {
    const column = geometryColumn.value;
    if (!column) {
      toast.error("Cette table n'a pas de colonne géométrique : rien ne peut être enregistré.");
      return;
    }
    const { updates, inserts } = changes;
    // The attributes collected before the session belong to the first shape
    // drawn in it. Saving without one would drop them silently, and the row
    // the user asked for would never exist (§8.3).
    if (pendingRow.value && inserts.length === 0) {
      toast.error("La nouvelle ligne attend sa géométrie : dessinez-la avant d'enregistrer, ou annulez.");
      return;
    }
    if (updates.size === 0 && inserts.length === 0) {
      reset();
      return;
    }

    saving.value = true;
    try {
      // Every moved geometry in one request: the server writes them in a
      // single transaction, so a shared limit is never half-written.
      if (updates.size > 0) {
        await updateRows(
          tableId.value,
          [...updates].map(([pkey, geometry]) => ({ pkey, values: { [column]: geometry } })),
        );
      }
      if (inserts.length > 0) {
        // Anything drawn after the first shape is geometry alone, and its
        // columns are filled from the grid.
        await addRows(
          tableId.value,
          inserts.map((geometry, index) => ({ ...(index === 0 ? draftValues.value : {}), [column]: geometry })),
        );
      }
      toast.success(summarizeWrites(updates.size, inserts.length));
      reset();
      onMutated();
    } catch (e) {
      toast.error(errorMessage(e, "Impossible d'enregistrer les géométries"));
    } finally {
      saving.value = false;
    }
  }

  function cancel() {
    // Dropping a session that was opened to give a new row its geometry drops
    // the row too — the user typed those attributes and has to be told they
    // went nowhere.
    if (pendingRow.value) toast.error("Ligne non créée : sa géométrie n'a pas été dessinée.");
    reset();
  }

  // A session belongs to the table it was opened on: its pkeys and its draft
  // mean nothing in another one (§9).
  watch(tableId, reset);

  return {
    active,
    saving,
    draftValues,
    prompt,
    loading: loaded.loading,
    features: loaded.features,
    truncated: loaded.truncated,
    loadIn: loaded.loadIn,
    start,
    save,
    cancel,
  };
}
