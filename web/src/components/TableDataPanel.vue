<script setup lang="ts">
import { computed, defineAsyncComponent, h, ref, toRef } from "vue";
import type { ColumnDef } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import type { ColumnSchema, Row, RowValue } from "../client/types";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useRowFilter } from "../composables/useRowFilter";
import { useTableRows } from "../composables/useTableRows";
import { PencilLine, Plus, SlidersHorizontal } from "@lucide/vue";
import DataTable from "./DataTable.vue";
import FilterConditionsEditor from "./FilterConditionsEditor.vue";
import FilterPills from "./FilterPills.vue";
import AddRowModal from "./AddRowModal.vue";
import TableRowEditCell from "./tables/TableRowEditCell.vue";
import TableRowActions from "./tables/TableRowActions.vue";
import BaseButton from "./ui/BaseButton.vue";

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
const addRowModalRef = ref<InstanceType<typeof AddRowModal>>();

function openAddRow() {
  showAddRow.value = true;
  addRowModalRef.value?.openModal();
}

async function onAddRow(values: Record<string, string>) {
  await rowsState.addRow(values);
}

const rowsColumnHelper = createColumnHelper<Row>();
const rowsColumnDefs = computed(() => {
  const defs: ColumnDef<Row, RowValue>[] = props.visibleColumns.map((col) =>
    rowsColumnHelper.accessor((row) => cellValue(row, col), {
      id: col,
      header: col,
      cell: (info) => {
        const isEditing = rowsState.editingPkey.value === info.row.original.pkey;
        const v = info.getValue();
        if (isEditing) {
          return h(TableRowEditCell, {
            modelValue: rowsState.editValues[col] ?? "",
            'onUpdate:modelValue': (val: string) => (rowsState.editValues[col] = val),
          });
        }
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
          return h(TableRowActions, {
            isEditing: rowsState.editingPkey.value === row.pkey,
            isSaving: rowsState.savingRow.value,
            isDeletingPkey: rowsState.deletingPkey.value ?? undefined,
            deletingPkey: row.pkey,
            onSave: rowsState.saveEditRow,
            onCancel: rowsState.cancelEditRow,
            onEdit: () => rowsState.startEditRow(row),
            onDelete: () => rowsState.deleteRow(row.pkey),
          });
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
      <BaseButton variant="secondary" class="flex items-center gap-1.5" @click="showFilterPanel = !showFilterPanel">
        <SlidersHorizontal :size="14" /> Filtres
        <span v-if="filter.appliedConditions.value.length > 0" class="rounded-full bg-indigo-600 px-1.5 text-xs font-semibold text-white">
          {{ filter.appliedConditions.value.length }}
        </span>
      </BaseButton>
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
        <BaseButton v-if="rowsState.editMode.value" variant="primary" class="flex items-center gap-1.5" @click="openAddRow">
          <Plus :size="14" /> Ajouter une ligne
        </BaseButton>
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

  <AddRowModal
    v-if="showAddRow"
    ref="addRowModalRef"
    :editable-columns="editableColumns"
    :geometry-column="geometryColumn"
    @add="onAddRow"
    @close="showAddRow = false"
  />
</template>
