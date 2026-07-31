import { computed, ref, watch, type Ref } from "vue";
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
  const filterError = ref<string | null>(null);
  const filterResults = ref<Row[] | null>(null);

  async function loadFilterResults() {
    filtering.value = true;
    filterError.value = null;
    try {
      const res = await filterRows(tableId.value, appliedConditions.value, filterLimit.value);
      filterResults.value = res.rows;
    } catch (e) {
      filterError.value = errorMessage(e, "Impossible d'appliquer le filtre");
    } finally {
      filtering.value = false;
    }
  }

  watch([appliedConditions, filterLimit], () => {
    if (filterActive.value) {
      loadFilterResults();
    } else {
      filterResults.value = null;
      filterError.value = null;
    }
  });

  // Re-centers the map on the filter's own results rather than the whole
  // table's extent, every time the filter is created/edited — cleared back
  // to the table's bbox once the filter is deactivated.
  const filteredBbox = ref<string | null>(null);

  async function refreshFilteredBbox() {
    if (!isGeo.value || !filterActive.value) {
      filteredBbox.value = null;
      return;
    }
    const { bbox } = await getFilteredExtent(tableId.value, appliedConditions.value).catch(() => ({ bbox: "" }));
    filteredBbox.value = bbox || null;
  }

  watch(appliedConditions, refreshFilteredBbox, { immediate: true });

  return { conditions, appliedConditions, filterActive, filterLimit, filtering, filterError, filterResults, filteredBbox };
}
