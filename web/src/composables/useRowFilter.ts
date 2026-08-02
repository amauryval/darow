import { computed, ref, type Ref } from "vue";
import { watchDebounced } from "@vueuse/core";
import { getFilteredExtent } from "../client/geo";
import { filterRows } from "../client/tables";
import type { ColumnSchema, FilterCondition, Row } from "../client/types";
import { errorMessage } from "../client/http";
import { coerceByType, columnType } from "../utils/columnTypes";
import { type DraftNode, pruneActive } from "../utils/filterTree";

// The filter tree editable by the UI, projected to the server DSL and the
// results/extent it produces. Only the active conditions are effective —
// they drive both the map (whichever of MVT/WFS is active renders the
// filtered rows) and the flat filtered-rows table, live, as soon as a pill
// is added/toggled/removed. Knows nothing of MapLibre or the paginated rows
// grid: its outputs (appliedConditions, filteredBbox) are consumed by
// whoever composes this with useMapLibre/useTableRows.
export function useRowFilter(tableId: Ref<string>, columns: Ref<ColumnSchema[]>, isGeo: Ref<boolean>) {
  const conditions = ref<DraftNode[]>([]);
  const appliedConditions = computed<FilterCondition[]>(() =>
    pruneActive(conditions.value, (col, v) => coerceByType(columnType(columns.value, col), v)),
  );
  const filterActive = computed(() => appliedConditions.value.length > 0);
  const filterLimit = ref(50);
  const filtering = ref(false);
  const filterError = ref<string>("");
  const filterResults = ref<Row[] | null>(null);

  async function loadFilterResults() {
    filtering.value = true;
    filterError.value = "";
    try {
      const res = await filterRows(tableId.value, appliedConditions.value, filterLimit.value);
      filterResults.value = res.rows;
    } catch (e) {
      filterError.value = errorMessage(e, "Impossible d'appliquer le filtre");
      // A filter that fails leaves no result set: showing the previous one
      // under a new filter would be a silent lie about what is on screen.
      filterResults.value = null;
    } finally {
      filtering.value = false;
    }
  }

  // Re-runs the current filter after a failure, so the user retries without
  // reloading the page (§8.1).
  function retry() {
    if (filterActive.value) loadFilterResults();
  }

  watchDebounced(
    [appliedConditions, filterLimit],
    () => {
      if (filterActive.value) {
        loadFilterResults();
      } else {
        filterResults.value = null;
        filterError.value = "";
      }
    },
    { debounce: 300 },
  );

  // Re-centers the map on the filter's own results rather than the whole
  // table's extent, every time the filter is created/edited — cleared back
  // to the table's bbox once the filter is deactivated.
  const filteredBbox = ref<string | null>(null);

  async function refreshFilteredBbox() {
    if (!isGeo.value || !filterActive.value) {
      filteredBbox.value = null;
      return;
    }
    // Best-effort: a failure here just skips re-centering the map on the
    // filter's extent (filteredBbox stays null, the table's own extent
    // keeps framing it) — the tolerated silence spec/front/spec.md §0.13
    // names explicitly for an extent-related display load.
    const { bbox } = await getFilteredExtent(tableId.value, appliedConditions.value).catch(() => ({ bbox: "" }));
    filteredBbox.value = bbox || null;
  }

  watchDebounced(appliedConditions, refreshFilteredBbox, { debounce: 300, immediate: true });

  return { conditions, appliedConditions, filterActive, filterLimit, filtering, filterError, filterResults, filteredBbox, retry };
}
