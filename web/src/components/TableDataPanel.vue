<script setup lang="ts">
import { computed, defineAsyncComponent, h, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import type { ColumnDef, SortingState, Updater } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import { getFilteredExtent } from "../client/geo";
import { addRows, deleteRow, filterRows, getRowsByPkey, listRows, updateRow } from "../client/tables";
import type { ColumnSchema, FilterCondition, Row, RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import { coerceByType, columnType } from "../utils/columnTypes";
import { sortingToQuery } from "../utils/sort";
import { useToast } from "../composables/useToast";
import { PencilLine, Plus, SlidersHorizontal } from "@lucide/vue";
import { type DraftNode, pruneActive } from "../utils/filterTree";
import ConfirmButton from "./ConfirmButton.vue";
import DataTable from "./DataTable.vue";
import FilterConditionsEditor from "./FilterConditionsEditor.vue";
import FilterPills from "./FilterPills.vue";
import Modal from "./Modal.vue";

// MapLibre GL is the largest dependency in the bundle — load it only when
// a geo table's data panel is actually rendered.
const MapView = defineAsyncComponent(() => import("./MapView.vue"));

const props = defineProps<{
  tableId: string;
  bbox?: string;
  isGeo: boolean;
  rowColumns: string[];
  visibleColumns: string[];
  editableColumns: string[];
  columns: ColumnSchema[];
  geometryColumn?: string;
}>();
// Emitted after any row add/edit/delete, since that changes the table's
// rowCount — the parent owns table metadata and reloads it.
const emit = defineEmits<{ mutated: [] }>();
const toast = useToast();

function cellValue(row: Row, col: string): RowValue {
  // The physical column is named "__pkey", but Row objects key it as
  // "pkey" (router.rowToJSON pulls it from storage.Row's dedicated Pkey
  // field rather than its Values map) — map the two back together for
  // display rather than showing a blank cell.
  return col === "__pkey" ? row.pkey : row[col];
}

// --- Filter panel: only the active conditions are effective. They drive
// both the map (whichever of MVT/WFS is active renders the filtered rows)
// and the table below (switches it to the flat, read-only filterRows
// results) — live, as soon as a pill is added/toggled/removed. ---
const showFilterPanel = ref(false);
const conditions = ref<DraftNode[]>([]);
const appliedConditions = computed<FilterCondition[]>(() =>
  pruneActive(conditions.value, (col, v) => coerceByType(columnType(props.columns, col), v)),
);
const filterLimit = ref(50);
const filtering = ref(false);
const filterError = ref<string | null>(null);
const filterActive = computed(() => appliedConditions.value.length > 0);

async function loadFilterResults() {
  filtering.value = true;
  filterError.value = null;
  try {
    const res = await filterRows(props.tableId, appliedConditions.value, filterLimit.value);
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
// table's extent, every time the filter is created/edited — cleared back to
// the table's bbox once the filter is deactivated.
const filteredBbox = ref<string | null>(null);

async function refreshFilteredBbox() {
  if (!props.isGeo || !filterActive.value) {
    filteredBbox.value = null;
    return;
  }
  const { bbox } = await getFilteredExtent(props.tableId, appliedConditions.value).catch(() => ({ bbox: "" }));
  filteredBbox.value = bbox || null;
}

watch(appliedConditions, refreshFilteredBbox, { immediate: true });

const mapBbox = computed(() => filteredBbox.value ?? props.bbox);

// --- Map (geo tables only) ---
interface MapViewHandle {
  deselect: (pkey: string) => void;
  clearSelection: () => void;
}
const mapViewRef = ref<MapViewHandle | null>(null);
const selectedPkeys = ref<string[]>([]);

function onDeselect(pkey: string) {
  mapViewRef.value?.deselect(pkey);
}
function onClearSelection() {
  mapViewRef.value?.clearSelection();
}

// Rows selected on the map are pinned at the top of the main rows table
// (below) rather than shown in a dedicated side panel — fetched in one
// batched request per selection change, cached so rows that stay selected
// across changes aren't refetched.
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
      const { rows: fetched } = await getRowsByPkey(props.tableId, missing);
      for (const row of fetched) selectedRowCache.set(row.pkey, row);
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de charger les lignes sélectionnées"));
    }
  }
  selectedRows.value = pkeys.map((pkey) => selectedRowCache.get(pkey)).filter((row): row is Row => row !== undefined);
}

watch(selectedPkeys, refreshSelectedRows, { immediate: true });

// --- Filtered results mode: flat, read-only, capped at filterLimit ---
const filterResults = ref<Row[] | null>(null);
const filterColumnHelper = createColumnHelper<Row>();
const filterColumnDefs = computed(() =>
  props.visibleColumns.map((col) => filterColumnHelper.accessor((row) => cellValue(row, col), { id: col, header: col })),
);
const filterTable = useVueTable({
  get data() {
    return filterResults.value ?? [];
  },
  get columns() {
    return filterColumnDefs.value;
  },
  getCoreRowModel: getCoreRowModel(),
});

// --- Unfiltered mode: infinite-scroll, sortable, editable rows grid ---
const rows = ref<Row[]>([]);
const total = ref(0);
const offset = ref(0);
const limit = 20;
const loadingMoreRows = ref(false);
const rowsLoadedOnce = ref(false);
const sorting = ref<SortingState>([]);

async function loadMoreRows() {
  if (loadingMoreRows.value || (rowsLoadedOnce.value && rows.value.length >= total.value)) return;
  loadingMoreRows.value = true;
  try {
    const page = await listRows(props.tableId, offset.value, limit, sortingToQuery(sorting.value));
    rows.value = [...rows.value, ...page.rows];
    total.value = page.total;
    offset.value += page.rows.length;
    rowsLoadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les lignes"));
  } finally {
    loadingMoreRows.value = false;
  }
}

// A row add/edit/delete can shift which rows belong on which page, and
// re-fetching only the pages already loaded wouldn't re-derive the running
// total correctly either — resetting to page 1 is the simplest correct
// behavior, matching how infinite-scroll lists commonly settle after a
// mutation.
async function reloadRows() {
  rows.value = [];
  total.value = 0;
  offset.value = 0;
  rowsLoadedOnce.value = false;
  await loadMoreRows();
}

// Like reloadRows(), but fetches page 1 before touching rows.value, so the
// table keeps showing its current rows instead of flashing empty while a
// sort change's request is in flight.
async function reloadRowsSorted() {
  loadingMoreRows.value = true;
  try {
    const page = await listRows(props.tableId, 0, limit, sortingToQuery(sorting.value));
    rows.value = page.rows;
    total.value = page.total;
    offset.value = page.rows.length;
    rowsLoadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les lignes"));
  } finally {
    loadingMoreRows.value = false;
  }
}

// --- Edit mode: toggling reveals a per-row "Modifier" action instead of
// switching straight to a spreadsheet-like grid, matching the inline
// edit/cancel pattern used everywhere else in this app (rename, EPSG). ---
const editMode = ref(false);
const editingPkey = ref<string | null>(null);
const editValues = reactive<Record<string, string>>({});
const savingRow = ref(false);
const deletingPkey = ref<string | null>(null);

function startEditRow(row: Row) {
  editingPkey.value = row.pkey;
  for (const col of props.rowColumns) {
    const v = cellValue(row, col);
    editValues[col] = v === null || v === undefined ? "" : String(v);
  }
}

function cancelEditRow() {
  editingPkey.value = null;
}

async function saveEditRow() {
  if (!editingPkey.value) return;
  savingRow.value = true;
  try {
    const updates: Record<string, unknown> = {};
    for (const col of props.rowColumns) {
      updates[col] = coerceByType(columnType(props.columns, col), editValues[col] ?? "");
    }
    await updateRow(props.tableId, editingPkey.value, updates);
    editingPkey.value = null;
    await reloadRows();
    toast.success("Ligne mise à jour.");
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de mettre à jour la ligne"));
  } finally {
    savingRow.value = false;
  }
}

async function onDeleteRow(pkey: string) {
  deletingPkey.value = pkey;
  try {
    await deleteRow(props.tableId, pkey);
    await reloadRows();
    emit("mutated");
    toast.success("Ligne supprimée.");
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de supprimer la ligne"));
  } finally {
    deletingPkey.value = null;
  }
}

// --- Add row modal ---
const showAddRow = ref(false);
const newRowValues = reactive<Record<string, string>>({});
const addingRow = ref(false);
const addRowError = ref<string | null>(null);

function openAddRow() {
  for (const col of props.editableColumns) newRowValues[col] = "";
  addRowError.value = null;
  showAddRow.value = true;
}

async function confirmAddRow() {
  addingRow.value = true;
  addRowError.value = null;
  try {
    const values: Record<string, unknown> = {};
    for (const col of props.editableColumns) {
      values[col] = coerceByType(columnType(props.columns, col), newRowValues[col] ?? "");
    }
    await addRows(props.tableId, [values]);
    showAddRow.value = false;
    await reloadRows();
    emit("mutated");
    toast.success("Ligne ajoutée.");
  } catch (e) {
    addRowError.value = errorMessage(e, "Impossible d'ajouter la ligne");
  } finally {
    addingRow.value = false;
  }
}

const rowsColumnHelper = createColumnHelper<Row>();
const rowsColumnDefs = computed(() => {
  const defs: ColumnDef<Row, RowValue>[] = props.visibleColumns.map((col) =>
    rowsColumnHelper.accessor((row) => cellValue(row, col), {
      id: col,
      header: col,
      cell: (info) => {
        if (editingPkey.value === info.row.original.pkey) {
          return h("input", {
            type: "text",
            value: editValues[col],
            onInput: (e: Event) => (editValues[col] = (e.target as HTMLInputElement).value),
            class: "w-full rounded-md border border-slate-300 px-2 py-1 text-sm",
          });
        }
        const v = info.getValue();
        return v === null || v === undefined ? "" : String(v);
      },
    }),
  );
  if (editMode.value) {
    defs.push(
      rowsColumnHelper.display({
        id: "actions",
        header: "",
        meta: { align: "right" },
        cell: (info) => {
          const row = info.row.original;
          if (editingPkey.value === row.pkey) {
            return h("div", { class: "flex justify-end gap-2" }, [
              h(
                "button",
                {
                  class: "rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50",
                  disabled: savingRow.value,
                  onClick: saveEditRow,
                },
                savingRow.value ? "…" : "Enregistrer",
              ),
              h(
                "button",
                {
                  class: "rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50",
                  onClick: cancelEditRow,
                },
                "Annuler",
              ),
            ]);
          }
          return h("div", { class: "flex justify-end gap-2" }, [
            h(
              "button",
              { class: "text-xs font-medium text-indigo-600 hover:text-indigo-700", onClick: () => startEditRow(row) },
              "Modifier",
            ),
            h(ConfirmButton, { label: "Supprimer", busy: deletingPkey.value === row.pkey, onConfirm: () => onDeleteRow(row.pkey) }),
          ]);
        },
      }),
    );
  }
  return defs;
});

// Selected rows are pinned first, ahead of the regular (unfiltered) page of
// rows — deduplicated so a selected row already present in the loaded page
// isn't shown twice.
const displayRows = computed(() => {
  if (selectedRows.value.length === 0) return rows.value;
  const selectedSet = new Set(selectedRows.value.map((row) => row.pkey));
  return [...selectedRows.value, ...rows.value.filter((row) => !selectedSet.has(row.pkey))];
});

function rowClassFor(row: { original: Row }) {
  return props.isGeo && selectedPkeys.value.includes(row.original.pkey) ? "bg-amber-50 hover:bg-amber-100 cursor-pointer" : "";
}

function onRowClick(row: { original: Row }) {
  if (props.isGeo && selectedPkeys.value.includes(row.original.pkey)) onDeselect(row.original.pkey);
}

const rowsTable = useVueTable({
  get data() {
    return displayRows.value;
  },
  get columns() {
    return rowsColumnDefs.value;
  },
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualSorting: true,
  get pageCount() {
    return Math.ceil(total.value / limit);
  },
  state: {
    get sorting() {
      return sorting.value;
    },
  },
  onSortingChange: (updater: Updater<SortingState>) => {
    sorting.value = typeof updater === "function" ? updater(sorting.value) : updater;
    reloadRowsSorted();
  },
});

const rowsSentinel = ref<HTMLElement | null>(null);
let rowsObserver: IntersectionObserver | null = null;

onMounted(() => {
  rowsObserver = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) loadMoreRows();
    },
    { rootMargin: "200px" },
  );
  if (rowsSentinel.value) rowsObserver.observe(rowsSentinel.value);
  loadMoreRows();
});

onUnmounted(() => rowsObserver?.disconnect());
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-4">
    <div class="flex shrink-0 flex-wrap items-center gap-2">
      <button
        class="flex items-center gap-1.5 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        @click="showFilterPanel = !showFilterPanel"
      >
        <SlidersHorizontal :size="14" /> Filtres
        <span v-if="appliedConditions.length > 0" class="rounded-full bg-indigo-600 px-1.5 text-xs font-semibold text-white">
          {{ appliedConditions.length }}
        </span>
      </button>
      <FilterPills :nodes="conditions" @update:nodes="conditions = $event" />
    </div>

    <div v-if="isGeo" class="h-[50vh] shrink-0 overflow-hidden rounded-lg border border-slate-200">
      <MapView ref="mapViewRef" :table-id="tableId" :bbox="mapBbox" :conditions="appliedConditions" @selection-change="selectedPkeys = $event" />
    </div>

    <div class="flex min-h-0 flex-1 flex-col">
      <div v-if="!filterActive" class="mb-3 flex shrink-0 items-center justify-between">
        <div class="flex items-center gap-4">
          <label class="flex items-center gap-2 text-sm text-slate-600">
            <input v-model="editMode" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-300" />
            <PencilLine :size="14" /> Mode édition
          </label>
          <span v-if="isGeo && selectedPkeys.length > 0" class="flex items-center gap-2 text-xs text-slate-500">
            <span class="inline-block h-2.5 w-2.5 rounded-sm bg-amber-200"></span>
            {{ selectedPkeys.length }} sélectionnée(s)
            <button class="font-medium text-slate-400 hover:text-red-600" @click="onClearSelection">Vider</button>
          </span>
        </div>
        <button
          v-if="editMode"
          class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500"
          @click="openAddRow"
        >
          <Plus :size="14" /> Ajouter une ligne
        </button>
      </div>

      <DataTable
        v-if="filterActive"
        :table="filterTable"
        :row-class="rowClassFor"
        empty-message="Aucun résultat."
        @row-click="onRowClick"
      />
      <DataTable v-else :table="rowsTable" :loading="loadingMoreRows" :row-class="rowClassFor" empty-message="Aucune ligne." @row-click="onRowClick">
        <template #footer>
          <div ref="rowsSentinel" class="h-px"></div>
          <p v-if="loadingMoreRows" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
        </template>
      </DataTable>
    </div>
  </div>

  <Teleport v-if="showFilterPanel" to="body">
    <div class="fixed inset-0 z-30" @click="showFilterPanel = false" />
    <div class="fixed inset-y-0 right-0 z-40 w-80 overflow-y-auto border-l border-slate-200 bg-white p-4 shadow-lg">
      <FilterConditionsEditor :columns="rowColumns" :nodes="conditions" @update:nodes="conditions = $event" />
      <div class="mt-4 flex items-center gap-2">
        <label class="text-xs text-slate-500">Limite (tableau filtré)</label>
        <input v-model.number="filterLimit" type="number" min="1" class="w-20 rounded-md border border-slate-300 px-2 py-1 text-xs" />
      </div>
      <p v-if="filterError" class="mt-2 text-xs text-red-600">{{ filterError }}</p>
    </div>
  </Teleport>

  <Modal v-if="showAddRow" title="Ajouter une ligne" @close="showAddRow = false">
    <div class="flex max-h-96 flex-col gap-3 overflow-y-auto pr-1">
      <label v-for="col in editableColumns" :key="col" class="block">
        <span class="mb-1 block text-xs font-medium text-slate-500">
          {{ col }} <span v-if="col === geometryColumn">(WKT)</span>
        </span>
        <input
          v-model="newRowValues[col]"
          type="text"
          :placeholder="col === geometryColumn ? 'POINT(2.35 48.85)' : ''"
          class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
        />
      </label>
    </div>
    <p v-if="addRowError" class="mt-3 text-xs text-red-600">{{ addRowError }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <button
        class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        @click="showAddRow = false"
      >
        Annuler
      </button>
      <button
        class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
        :disabled="addingRow"
        @click="confirmAddRow"
      >
        {{ addingRow ? "Ajout…" : "Ajouter" }}
      </button>
    </div>
  </Modal>
</template>
