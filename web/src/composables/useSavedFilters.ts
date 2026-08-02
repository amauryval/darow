import { ref, watch, type Ref } from "vue";
import { createSavedFilter, deleteSavedFilter, listSavedFilters, updateSavedFilter } from "../client/filters";
import { errorMessage } from "../client/http";
import type { FilterCondition, SavedFilter } from "../client/types";

// The filters persisted server-side against one table. Holds the list and its
// four states; every mutation reloads from the server rather than patching the
// local list, since the server owns the identifiers and the timestamps
// (spec/front/spec.md §8.2 — no optimistic UI). Knows nothing of the draft
// tree the editor manipulates: the caller converts between the two through
// utils/filterTree.
export function useSavedFilters(tableId: Ref<string>) {
  const filters = ref<SavedFilter[]>([]);
  const loading = ref(false);
  const loadedOnce = ref(false);
  const error = ref("");
  const saving = ref(false);

  async function load() {
    loading.value = true;
    error.value = "";
    try {
      const page = await listSavedFilters(tableId.value);
      filters.value = page.filters;
      loadedOnce.value = true;
    } catch (e) {
      error.value = errorMessage(e, "Impossible de charger les filtres enregistrés");
    } finally {
      loading.value = false;
    }
  }

  // A parameterized view is reused when only tableId changes, so the initial
  // load is a watch rather than onMounted (spec/front/spec.md §9).
  watch(
    tableId,
    () => {
      filters.value = [];
      loadedOnce.value = false;
      load();
    },
    { immediate: true },
  );

  // The three mutations rethrow after reloading: the caller decides between a
  // toast and an inline message, and needs to know whether to close its modal.
  async function mutate<T>(action: () => Promise<T>): Promise<T> {
    saving.value = true;
    try {
      const result = await action();
      await load();
      return result;
    } finally {
      saving.value = false;
    }
  }

  function create(name: string, conditions: FilterCondition[]): Promise<SavedFilter> {
    return mutate(() => createSavedFilter(tableId.value, name, conditions));
  }

  function update(filterId: string, patch: { name?: string; conditions?: FilterCondition[] }): Promise<SavedFilter> {
    return mutate(() => updateSavedFilter(tableId.value, filterId, patch));
  }

  function remove(filterId: string): Promise<void> {
    return mutate(() => deleteSavedFilter(tableId.value, filterId));
  }

  return { filters, loading, loadedOnce, error, saving, load, create, update, remove };
}
