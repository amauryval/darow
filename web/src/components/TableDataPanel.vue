<script setup lang="ts">
import { computed, defineAsyncComponent, h, ref, toRef, watch } from "vue";
import type { ColumnDef } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import type { ColumnSchema, Row, RowValue } from "../client/types";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useRowFilter } from "../composables/useRowFilter";
import { useTableRows } from "../composables/useTableRows";
import { cellValue } from "../utils/rows";
import { LayoutPanelLeft, Map as MapIcon, PencilLine, Plus, Table2 } from "@lucide/vue";
import DataTable from "./DataTable.vue";
import FilteredRowsPanel from "./filters/FilteredRowsPanel.vue";
import TableFilterBar from "./filters/TableFilterBar.vue";
import AddRowModal from "./AddRowModal.vue";
import TableRowEditCell from "./tables/TableRowEditCell.vue";
import TableRowActions from "./tables/TableRowActions.vue";
import BaseButton from "./ui/BaseButton.vue";
import BaseCheckbox from "./ui/BaseCheckbox.vue";
import ErrorState from "./ui/ErrorState.vue";
import Skeleton from "./ui/Skeleton.vue";

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

// Table and map share the space 50/50 by default; either can be maximized to
// take the whole width, restored back to 'split' from the same toggle group.
const focusPanel = ref<"split" | "table" | "map">("split");
watch(tableId, () => (focusPanel.value = "split"));

// Re-centers the map on the filter's own results rather than the whole
// table's extent, cleared back to the table's bbox once inactive.
const mapBbox = computed(() => filter.filteredBbox.value ?? props.bbox);

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
  return props.isGeo && selection.selectedPkeys.value.includes(row.original.pkey) ? "bg-warning/10 hover:bg-warning/20 cursor-pointer" : "";
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
        <TableFilterBar
          :table-id="tableId"
          :columns="columns"
          @update:nodes="filter.conditions.value = $event"
        />

        <!-- Table and map default to half the width each; either can be
             maximized when the data being read needs the room (§8.10). -->
        <div v-if="isGeo" class="join ml-auto">
          <BaseButton
            class="join-item"
            size="sm"
            :variant="focusPanel === 'table' ? 'primary' : 'secondary'"
            :icon="Table2"
            :aria-pressed="focusPanel === 'table'"
            @click="focusPanel = focusPanel === 'table' ? 'split' : 'table'"
          >
            Tableau
          </BaseButton>
          <BaseButton
            class="join-item"
            size="sm"
            :variant="focusPanel === 'split' ? 'primary' : 'secondary'"
            :icon="LayoutPanelLeft"
            icon-only
            aria-label="Partager la vue"
            title="Partager la vue"
            @click="focusPanel = 'split'"
          />
          <BaseButton
            class="join-item"
            size="sm"
            :variant="focusPanel === 'map' ? 'primary' : 'secondary'"
            :icon="MapIcon"
            :aria-pressed="focusPanel === 'map'"
            @click="focusPanel = focusPanel === 'map' ? 'split' : 'map'"
          >
            Carte
          </BaseButton>
        </div>
      </div>

      <div class="flex min-h-0 flex-1 gap-4">
        <div
          v-if="!isGeo || focusPanel !== 'map'"
          class="flex min-h-0 flex-col"
          :class="isGeo ? (focusPanel === 'table' ? 'w-full' : 'w-1/2') : 'w-full flex-1'"
        >
          <div v-if="!filter.filterActive.value" class="mb-3 flex shrink-0 items-center justify-between">
            <div class="flex items-center gap-4">
              <label class="flex items-center gap-2 text-sm">
                <BaseCheckbox v-model="rowsState.editMode.value" />
                <PencilLine :size="14" /> Mode édition
              </label>
              <span v-if="isGeo && selection.selectedPkeys.value.length > 0" class="flex items-center gap-2 text-xs text-base-content/60">
                <span class="badge badge-warning badge-xs"></span>
                {{ selection.selectedPkeys.value.length }} sélectionnée(s)
                <BaseButton variant="ghost" size="xs" class="text-base-content/50 hover:text-error" @click="selection.clear">Vider</BaseButton>
              </span>
            </div>
            <BaseButton v-if="rowsState.editMode.value" variant="primary" class="flex items-center gap-1.5" @click="openAddRow">
              <Plus :size="14" /> Ajouter une ligne
            </BaseButton>
          </div>

          <FilteredRowsPanel
            v-if="filter.filterActive.value"
            key="filter"
            :visible-columns="visibleColumns"
            :rows="filter.filterResults.value"
            :loading="filter.filtering.value"
            :error="filter.filterError.value"
            :limit="filter.filterLimit.value"
            :row-class="rowClassFor"
            @update:limit="filter.filterLimit.value = $event"
            @retry="filter.retry"
            @row-click="onRowClick"
          />
          <Skeleton v-else-if="rowsState.loading.value && !rowsState.loadedOnce.value" :rows="8" />
          <ErrorState v-else-if="rowsState.error.value" :message="rowsState.error.value" @retry="rowsState.retry" />
          <DataTable
            v-else
            key="rows"
            :table="rowsTable"
            :loading="rowsState.loading.value"
            :row-class="rowClassFor"
            empty-message="Aucune ligne."
            @row-click="onRowClick"
          >
            <template #footer>
              <div :ref="rowsState.sentinel" class="h-px"></div>
              <p v-if="rowsState.loading.value" class="px-4 py-3 text-center text-xs text-base-content/40">Chargement…</p>
            </template>
          </DataTable>
        </div>

        <div
          v-if="isGeo && focusPanel !== 'table'"
          class="card min-h-0 overflow-hidden rounded-lg border border-base-300"
          :class="focusPanel === 'map' ? 'w-full' : 'w-1/2'"
        >
          <MapView
            :table-id="tableId"
            :bbox="mapBbox"
            :conditions="filter.appliedConditions.value"
            :selected-pkeys="selection.selectedPkeys.value"
            @toggle-selection="selection.toggle"
          />
        </div>
      </div>
    </div>

  <AddRowModal
    v-if="showAddRow"
    ref="addRowModalRef"
    :editable-columns="editableColumns"
    :geometry-column="geometryColumn"
    @add="onAddRow"
    @close="showAddRow = false"
  />
</template>
