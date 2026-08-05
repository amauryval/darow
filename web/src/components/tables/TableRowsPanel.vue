<script setup lang="ts">
import { computed, h } from "vue";
import type { ColumnDef } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import { Plus } from "@lucide/vue";
import type { Row, RowValue } from "../../client/types";
import type { useTableRows } from "../../composables/useTableRows";
import { cellValue } from "../../utils/rows";
import DataTable from "../DataTable.vue";
import TableRowActions from "./TableRowActions.vue";
import TableRowEditCell from "./TableRowEditCell.vue";
import BaseButton from "../ui/BaseButton.vue";
import ErrorState from "../ui/ErrorState.vue";
import Skeleton from "../ui/Skeleton.vue";
import Typography from "../ui/Typography.vue";

// The rows grid of a table: the pinned selection, the infinite scroll, and
// the cells the attribute edit mode makes writable. It receives the row state
// its view already owns (useTableRows) and reports intentions; it loads
// nothing itself, and it does not own the mode either — arming it is a
// table-level action, next to the one that arms the geometries (§8.8).
const props = defineProps<{
  state: ReturnType<typeof useTableRows>;
  displayRows: Row[];
  visibleColumns: string[];
  isGeo: boolean;
  selectedCount: number;
  rowClass?: (row: { original: Row }) => string;
}>();
const emit = defineEmits<{
  addRow: [];
  updateCell: [column: string, value: string];
  geometry: [pkey: string];
  clearSelection: [];
  rowClick: [row: { original: Row }];
}>();

const columnHelper = createColumnHelper<Row>();
const columnDefs = computed(() => {
  const defs: ColumnDef<Row, RowValue>[] = props.visibleColumns.map((col) =>
    columnHelper.accessor((row) => cellValue(row, col), {
      id: col,
      header: col,
      cell: (info) => {
        const isEditing = props.state.editingPkey.value === info.row.original.pkey;
        const v = info.getValue();
        if (isEditing) {
          return h(TableRowEditCell, {
            modelValue: props.state.editValues[col] ?? "",
            "onUpdate:modelValue": (val: string) => emit("updateCell", col, val),
          });
        }
        return v === null || v === undefined ? "" : String(v);
      },
    }),
  );

  // The actions column is always there: reading a row's geometry is a read,
  // and arming edit mode to look at one would be a trap.
  defs.push(
    columnHelper.display({
      id: "actions",
      header: "",
      meta: { align: "right" },
      cell: (info) => {
        const row = info.row.original;
        return h(TableRowActions, {
          editable: props.state.editMode.value,
          isGeo: props.isGeo,
          isEditing: props.state.editingPkey.value === row.pkey,
          isSaving: props.state.savingRow.value,
          isDeletingPkey: props.state.deletingPkey.value ?? undefined,
          deletingPkey: row.pkey,
          onSave: props.state.saveEditRow,
          onCancel: props.state.cancelEditRow,
          onEdit: () => props.state.startEditRow(row),
          onDelete: () => props.state.deleteRow(row.pkey),
          onGeometry: () => emit("geometry", row.pkey),
        });
      },
    }),
  );
  return defs;
});

const table = useVueTable({
  get data() {
    return props.displayRows;
  },
  get columns() {
    return columnDefs.value;
  },
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualSorting: true,
  get pageCount() {
    return Math.ceil(props.state.total.value / props.state.pageSize);
  },
  state: {
    get sorting() {
      return props.state.sorting.value;
    },
  },
  onSortingChange: props.state.setSorting,
});
</script>

<template>
  <div class="mb-3 flex shrink-0 items-center justify-between">
    <div class="flex items-center gap-4">
      <span v-if="isGeo && selectedCount > 0" class="flex items-center gap-2">
        <span class="badge badge-warning badge-xs"></span>
        <Typography variant="caption">{{ selectedCount }} sélectionnée(s)</Typography>
        <BaseButton variant="ghost" size="xs" @click="emit('clearSelection')">Vider</BaseButton>
      </span>
    </div>
    <BaseButton v-if="state.editMode.value" variant="primary" size="sm" :icon="Plus" @click="emit('addRow')">
      Ajouter une ligne
    </BaseButton>
  </div>

  <Skeleton v-if="state.loading.value && !state.loadedOnce.value" :rows="8" />
  <ErrorState v-else-if="state.error.value" :message="state.error.value" @retry="state.retry" />
  <DataTable
    v-else
    :table="table"
    :loading="state.loading.value"
    :row-class="rowClass"
    empty-message="Aucune ligne."
    @row-click="emit('rowClick', $event)"
  >
    <template #footer>
      <div :ref="state.sentinel" class="h-px"></div>
      <p v-if="state.loading.value" class="px-4 py-3 text-center text-xs text-base-content/40">Chargement…</p>
    </template>
  </DataTable>
</template>
