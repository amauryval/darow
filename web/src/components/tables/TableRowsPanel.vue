<script setup lang="ts">
import { computed, h } from "vue";
import type { ColumnDef } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import type { Row, RowValue } from "../../client/types";
import { isDraft } from "../../composables/useRowEdits";
import type { useRowEdits } from "../../composables/useRowEdits";
import type { useTableRows } from "../../composables/useTableRows";
import { cellValue } from "../../utils/rows";
import DataTable from "../DataTable.vue";
import TableRowActions from "./TableRowActions.vue";
import TableRowEditCell from "./TableRowEditCell.vue";
import ErrorState from "../ui/ErrorState.vue";
import Skeleton from "../ui/Skeleton.vue";

// The rows grid of a table: the pinned selection, the infinite scroll, and
// the cells the attribute edit mode makes writable. It receives the row state
// its view already owns (useTableRows) and reports intentions; it loads
// nothing itself, and it does not own the mode either — arming it is a
// table-level action, next to the one that arms the geometries (§8.8).
const props = defineProps<{
  state: ReturnType<typeof useTableRows>;
  // The session's attribute drafts. Which columns it may write is not the
  // grid's to guess: the geometry is drawn, never typed, and a column the
  // table does not own (the pkey) is not a field either.
  edits: ReturnType<typeof useRowEdits>;
  editableColumns: string[];
  displayRows: Row[];
  visibleColumns: string[];
  isGeo: boolean;
  rowClass?: (row: { original: Row }) => string;
}>();
const emit = defineEmits<{
  geometry: [pkey: string];
  rowClick: [row: { original: Row }];
}>();

const columnHelper = createColumnHelper<Row>();
const columnDefs = computed(() => {
  const defs: ColumnDef<Row, RowValue>[] = props.visibleColumns.map((col) =>
    columnHelper.accessor((row) => cellValue(row, col), {
      id: col,
      header: col,
      cell: (info) => {
        const row = info.row.original;
        if (props.edits.active.value && props.editableColumns.includes(col)) {
          return h(TableRowEditCell, {
            modelValue: props.edits.display(row, col),
            column: col,
            dirty: props.edits.isDirty(row.pkey, col),
            "onUpdate:modelValue": (value: string) => props.edits.setCell(row, col, value),
          });
        }
        const v = info.getValue();
        return v === null || v === undefined ? "" : String(v);
      },
    }),
  );

  // The row's own two columns, at the **start** of the grid and pinned there:
  // its rank, and what a click does to it. Both stay put while the columns
  // scroll sideways — a grid wider than its panel otherwise hid the number
  // that names the row and pushed its actions past every attribute (§8.6).
  //
  // The rank is the position of the row in the grid as it is displayed —
  // selection pinned at the top included. It is what a user points at when
  // they say "the third one", and it is not the primary key: a pkey is an
  // identifier, not a position, and the two disagree as soon as the grid is
  // sorted.
  defs.unshift(
    columnHelper.display({
      id: "actions",
      header: "",
      // Width of its content and nothing more (`w-px` + `whitespace-nowrap`,
      // the one way a table cell shrinks to fit): the column holds one icon
      // outside an edit session and three under one, and a fixed width sized
      // for three left a hand's width of nothing between the row number and
      // the first attribute.
      meta: { pinned: "left-12", class: "w-px px-2 whitespace-nowrap" },
      cell: (info) => {
        const row = info.row.original;
        // A row that does not exist yet has nothing to duplicate and no
        // geometry to read. Dropping it is the one thing that applies — and
        // only where the grid alone created it: a draft raised by a drawing
        // is dropped with the shape, which is the session's *Annuler*.
        if (isDraft(row.pkey)) {
          return props.isGeo
            ? null
            : h(TableRowActions, { draft: true, onDelete: () => props.edits.removeDraft(row.pkey) });
        }
        return h(TableRowActions, {
          editable: props.edits.active.value,
          isGeo: props.isGeo,
          isDeletingPkey: props.state.deletingPkey.value ?? undefined,
          isDuplicating: props.state.duplicatingPkey.value === row.pkey,
          deletingPkey: row.pkey,
          onDuplicate: () => props.state.duplicateRow(row),
          onDelete: () => props.state.deleteRow(row.pkey),
          onGeometry: () => emit("geometry", row.pkey),
        });
      },
    }),
  );

  defs.unshift(
    columnHelper.display({
      id: "__rownum",
      header: "#",
      meta: { align: "right", pinned: "left-0", class: "w-12 min-w-12 px-2 tabular-nums text-base-content/40" },
      // The drafts sit above the grid's own rows and are not numbered: they
      // hold no position in the table until they are written, and numbering
      // them would shift every rank the user reads by however many of them
      // are open.
      cell: (info) =>
        isDraft(info.row.original.pkey) ? "" : String(info.row.index + 1 - props.edits.draftCount.value),
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
