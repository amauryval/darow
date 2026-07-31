<script setup lang="ts">
import { computed, defineAsyncComponent, h, reactive, ref, toRef } from "vue";
import type { ColumnDef } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import type { ColumnSchema, Row, RowValue } from "../client/types";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useRowFilter } from "../composables/useRowFilter";
import { useTableRows } from "../composables/useTableRows";
import { PencilLine, Plus, SlidersHorizontal } from "@lucide/vue";
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

const tableId = toRef(props, "tableId");
const columns = toRef(props, "columns");
const isGeo = toRef(props, "isGeo");

const filter = useRowFilter(tableId, columns, isGeo);
const selection = useFeatureSelection(tableId);
const rowsState = useTableRows(tableId, toRef(props, "rowColumns"), toRef(props, "editableColumns"), columns, () => emit("mutated"));

const showFilterPanel = ref(false);
// Re-centers the map on the filter's own results rather than the whole
// table's extent, cleared back to the table's bbox once inactive.
const mapBbox = computed(() => filter.filteredBbox.value ?? props.bbox);

function cellValue(row: Row, col: string): RowValue {
  return rowsState.cellValue(row, col);
}

// --- Filtered results mode: flat, read-only, capped at filterLimit ---
const filterColumnHelper = createColumnHelper<Row>();
const filterColumnDefs = computed(() =>
  props.visibleColumns.map((col) => filterColumnHelper.accessor((row) => cellValue(row, col), { id: col, header: col })),
);
const filterTable = useVueTable({
  get data() {
    return filter.filterResults.value ?? [];
  },
  get columns() {
    return filterColumnDefs.value;
  },
  getCoreRowModel: getCoreRowModel(),
});

// --- Add row modal ---
const showAddRow = ref(false);
const newRowValues = reactive<Record<string, string>>({});

function openAddRow() {
  for (const col of props.editableColumns) newRowValues[col] = "";
  showAddRow.value = true;
}

async function confirmAddRow() {
  if (await rowsState.addRow(newRowValues)) showAddRow.value = false;
}

const rowsColumnHelper = createColumnHelper<Row>();
const rowsColumnDefs = computed(() => {
  const defs: ColumnDef<Row, RowValue>[] = props.visibleColumns.map((col) =>
    rowsColumnHelper.accessor((row) => cellValue(row, col), {
      id: col,
      header: col,
      cell: (info) => {
        if (rowsState.editingPkey.value === info.row.original.pkey) {
          return h("input", {
            type: "text",
            value: rowsState.editValues[col],
            onInput: (e: Event) => (rowsState.editValues[col] = (e.target as HTMLInputElement).value),
            class: "w-full rounded-md border border-slate-300 px-2 py-1 text-sm",
          });
        }
        const v = info.getValue();
        return v === null || v === undefined ? "" : String(v);
      },
    }),
  );
  if (rowsState.editMode.value) {
    defs.push(
      rowsColumnHelper.display({
        id: "actions",
        header: "",
        meta: { align: "right" },
        cell: (info) => {
          const row = info.row.original;
          if (rowsState.editingPkey.value === row.pkey) {
            return h("div", { class: "flex justify-end gap-2" }, [
              h(
                "button",
                {
                  class: "rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50",
                  disabled: rowsState.savingRow.value,
                  onClick: rowsState.saveEditRow,
                },
                rowsState.savingRow.value ? "…" : "Enregistrer",
              ),
              h(
                "button",
                {
                  class: "rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50",
                  onClick: rowsState.cancelEditRow,
                },
                "Annuler",
              ),
            ]);
          }
          return h("div", { class: "flex justify-end gap-2" }, [
            h(
              "button",
              { class: "text-xs font-medium text-indigo-600 hover:text-indigo-700", onClick: () => rowsState.startEditRow(row) },
              "Modifier",
            ),
            h(ConfirmButton, {
              label: "Supprimer",
              busy: rowsState.deletingPkey.value === row.pkey,
              onConfirm: () => rowsState.deleteRow(row.pkey),
            }),
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
  if (selection.selectedRows.value.length === 0) return rowsState.rows.value;
  const selectedSet = new Set(selection.selectedRows.value.map((row) => row.pkey));
  return [...selection.selectedRows.value, ...rowsState.rows.value.filter((row) => !selectedSet.has(row.pkey))];
});

function rowClassFor(row: { original: Row }) {
  return props.isGeo && selection.selectedPkeys.value.includes(row.original.pkey) ? "bg-amber-50 hover:bg-amber-100 cursor-pointer" : "";
}

function onRowClick(row: { original: Row }) {
  if (props.isGeo && selection.selectedPkeys.value.includes(row.original.pkey)) selection.deselect(row.original.pkey);
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
    return Math.ceil(rowsState.total.value / rowsState.pageSize);
  },
  state: {
    get sorting() {
      return rowsState.sorting.value;
    },
  },
  onSortingChange: rowsState.setSorting,
});
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-4">
    <div class="flex shrink-0 flex-wrap items-center gap-2">
      <button
        class="flex items-center gap-1.5 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        @click="showFilterPanel = !showFilterPanel"
      >
        <SlidersHorizontal :size="14" /> Filtres
        <span v-if="filter.appliedConditions.value.length > 0" class="rounded-full bg-indigo-600 px-1.5 text-xs font-semibold text-white">
          {{ filter.appliedConditions.value.length }}
        </span>
      </button>
      <FilterPills :nodes="filter.conditions.value" @update:nodes="filter.conditions.value = $event" />
    </div>

    <div v-if="isGeo" class="h-[50vh] shrink-0 overflow-hidden rounded-lg border border-slate-200">
      <MapView
        :table-id="tableId"
        :bbox="mapBbox"
        :conditions="filter.appliedConditions.value"
        :selected-pkeys="selection.selectedPkeys.value"
        @toggle-selection="selection.toggle"
      />
    </div>

    <div class="flex min-h-0 flex-1 flex-col">
      <div v-if="!filter.filterActive.value" class="mb-3 flex shrink-0 items-center justify-between">
        <div class="flex items-center gap-4">
          <label class="flex items-center gap-2 text-sm text-slate-600">
            <input v-model="rowsState.editMode.value" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-300" />
            <PencilLine :size="14" /> Mode édition
          </label>
          <span v-if="isGeo && selection.selectedPkeys.value.length > 0" class="flex items-center gap-2 text-xs text-slate-500">
            <span class="inline-block h-2.5 w-2.5 rounded-sm bg-amber-200"></span>
            {{ selection.selectedPkeys.value.length }} sélectionnée(s)
            <button class="font-medium text-slate-400 hover:text-red-600" @click="selection.clear">Vider</button>
          </span>
        </div>
        <button
          v-if="rowsState.editMode.value"
          class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500"
          @click="openAddRow"
        >
          <Plus :size="14" /> Ajouter une ligne
        </button>
      </div>

      <DataTable
        v-if="filter.filterActive.value"
        :table="filterTable"
        :row-class="rowClassFor"
        empty-message="Aucun résultat."
        @row-click="onRowClick"
      />
      <DataTable
        v-else
        :table="rowsTable"
        :loading="rowsState.loading.value"
        :row-class="rowClassFor"
        empty-message="Aucune ligne."
        @row-click="onRowClick"
      >
        <template #footer>
          <div :ref="rowsState.sentinel" class="h-px"></div>
          <p v-if="rowsState.loading.value" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
        </template>
      </DataTable>
    </div>
  </div>

  <Teleport v-if="showFilterPanel" to="body">
    <div class="fixed inset-0 z-30" @click="showFilterPanel = false" />
    <div class="fixed inset-y-0 right-0 z-40 w-80 overflow-y-auto border-l border-slate-200 bg-white p-4 shadow-lg">
      <FilterConditionsEditor :columns="rowColumns" :nodes="filter.conditions.value" @update:nodes="filter.conditions.value = $event" />
      <div class="mt-4 flex items-center gap-2">
        <label class="text-xs text-slate-500">Limite (tableau filtré)</label>
        <input v-model.number="filter.filterLimit.value" type="number" min="1" class="w-20 rounded-md border border-slate-300 px-2 py-1 text-xs" />
      </div>
      <p v-if="filter.filterError.value" class="mt-2 text-xs text-red-600">{{ filter.filterError.value }}</p>
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
    <p v-if="rowsState.addRowError.value" class="mt-3 text-xs text-red-600">{{ rowsState.addRowError.value }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <button
        class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        @click="showAddRow = false"
      >
        Annuler
      </button>
      <button
        class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
        :disabled="rowsState.addingRow.value"
        @click="confirmAddRow"
      >
        {{ rowsState.addingRow.value ? "Ajout…" : "Ajouter" }}
      </button>
    </div>
  </Modal>
</template>
