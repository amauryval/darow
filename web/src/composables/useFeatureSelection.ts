import { ref, watch, type Ref } from "vue";
import { getRowsByPkey } from "../client/tables";
import type { Row } from "../client/types";
import { errorMessage } from "../client/http";
import { useToast } from "./useToast";

// Feature selection (pkeys) and batch hydration of the matching rows. Owns
// selectedPkeys as the single source of truth — a map composable (e.g.
// useMvtLayer) reflects it onto the map's styling by watching it, and
// calls toggle() on click, instead of the map and the row list negotiating
// selection through a component ref and an emit.
export function useFeatureSelection(tableId: Ref<string>) {
  const toast = useToast();
  const selectedPkeys = ref<string[]>([]);

  function toggle(pkey: string) {
    selectedPkeys.value = selectedPkeys.value.includes(pkey)
      ? selectedPkeys.value.filter((p) => p !== pkey)
      : [...selectedPkeys.value, pkey];
  }

  function clear() {
    selectedPkeys.value = [];
  }

  // Rows selected on the map are hydrated in one batched request per
  // selection change, cached so rows that stay selected across changes
  // aren't refetched.
  const selectedRowCache = new Map<string, Row>();
  const selectedRows = ref<Row[]>([]);

  async function refreshSelectedRows() {
    const pkeys = selectedPkeys.value;
    for (const cached of Array.from(selectedRowCache.keys())) {
      if (!pkeys.includes(cached)) selectedRowCache.delete(cached);
    }
    const missing = pkeys.filter((pkey) => !selectedRowCache.has(pkey));
    if (missing.length > 0) {
      try {
        const { rows: fetched } = await getRowsByPkey(tableId.value, missing);
        for (const row of fetched) selectedRowCache.set(row.pkey, row);
      } catch (e) {
        toast.error(errorMessage(e, "Impossible de charger les lignes sélectionnées"));
      }
    }
    selectedRows.value = pkeys.map((pkey) => selectedRowCache.get(pkey)).filter((row): row is Row => row !== undefined);
  }

  watch(selectedPkeys, refreshSelectedRows, { immediate: true });

  // A pkey only means something inside its own table: carried over to
  // another one it would be hydrated against rows that do not exist, and
  // highlight nothing on the map (§9).
  watch(tableId, () => {
    selectedRowCache.clear();
    clear();
  });

  return { selectedPkeys, selectedRows, toggle, clear };
}
