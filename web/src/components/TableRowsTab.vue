<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, reactive, ref } from "vue";
import type { ColumnDef, SortingState, Updater } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import { addRows, deleteRow, listRows, updateRow } from "../client/tables";
import type { ColumnSchema, Row, RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import { coerceByType, columnType } from "../utils/columnTypes";
import { sortingToQuery } from "../utils/sort";
import { useToast } from "../composables/useToast";
import { PencilLine, Plus } from "@lucide/vue";
import ConfirmButton from "./ConfirmButton.vue";
import DataTable from "./DataTable.vue";
import Modal from "./Modal.vue";

const props = defineProps<{
  tableId: string;
  rowColumns: string[];
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
  const defs: ColumnDef<Row, RowValue>[] = props.rowColumns.map((col) =>
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

const rowsTable = useVueTable({
  get data() {
    return rows.value;
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
  <div class="flex min-h-0 flex-1 flex-col">
    <div class="mb-3 flex shrink-0 items-center justify-between">
      <label class="flex items-center gap-2 text-sm text-slate-600">
        <input v-model="editMode" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-300" />
        <PencilLine :size="14" /> Mode édition
      </label>
      <button
        v-if="editMode"
        class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500"
        @click="openAddRow"
      >
        <Plus :size="14" /> Ajouter une ligne
      </button>
    </div>

    <DataTable :table="rowsTable" :loading="loadingMoreRows" empty-message="Aucune ligne.">
      <template #footer>
        <div ref="rowsSentinel" class="h-px"></div>
        <p v-if="loadingMoreRows" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
      </template>
    </DataTable>
  </div>

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
