<script setup lang="ts">
import { computed, h, ref, watch } from "vue";
import type { RowSelectionState, SortingState, Updater } from "@tanstack/vue-table";
import { FlexRender, createColumnHelper, getCoreRowModel, getSortedRowModel, useVueTable } from "@tanstack/vue-table";
import { getRowsByPkey } from "../client/tables";
import type { Row } from "../client/types";
import { errorMessage } from "../client/http";

const props = defineProps<{
  tableId: string;
  // Attribute columns to display — the caller excludes the geometry
  // column and the raw "__pkey" field (see TableDetailView's rowColumns).
  columns: string[];
  selectedPkeys: string[];
}>();

const emit = defineEmits<{ deselect: [pkey: string]; clear: [] }>();

const rowCache = new Map<string, Row>();
const rowsData = ref<Row[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);
const sorting = ref<SortingState>([]);

// Every selected row is fetched in a single batched request (getRowsByPkey)
// instead of one GetRow call per pkey — that also means the whole
// selection is in hand for client-side sorting, rather than only whatever
// page happened to be loaded. rowCache avoids refetching rows that stay
// selected across successive selection changes.
async function refreshRows() {
  const pkeys = props.selectedPkeys;
  for (const cached of Array.from(rowCache.keys())) {
    if (!pkeys.includes(cached)) rowCache.delete(cached);
  }
  const missing = pkeys.filter((pkey) => !rowCache.has(pkey));
  if (missing.length > 0) {
    loading.value = true;
    error.value = null;
    try {
      const { rows } = await getRowsByPkey(props.tableId, missing);
      for (const row of rows) rowCache.set(row.pkey, row);
    } catch (e) {
      error.value = errorMessage(e, "Impossible de charger les lignes sélectionnées");
    } finally {
      loading.value = false;
    }
  }
  rowsData.value = pkeys.map((pkey) => rowCache.get(pkey)).filter((row): row is Row => row !== undefined);
}

watch(() => props.selectedPkeys, refreshRows, { immediate: true });

// Every row shown here is, by definition, already part of the map
// selection — rowSelection starts fully checked and is kept in sync so
// TanStack's own selection state always mirrors it. Unchecking a row (or
// the header's "select all") is how the panel drives deselection back to
// the map, through onRowSelectionChange below.
const rowSelection = ref<RowSelectionState>({});
watch(rowsData, (rows) => {
  rowSelection.value = Object.fromEntries(rows.map((row) => [row.pkey, true]));
});

const columnHelper = createColumnHelper<Row>();
const columnDefs = computed(() => [
  columnHelper.display({
    id: "select",
    header: ({ table }) =>
      h("input", {
        type: "checkbox",
        checked: table.getIsAllRowsSelected(),
        onChange: table.getToggleAllRowsSelectedHandler(),
        class: "h-3.5 w-3.5 rounded border-slate-300",
      }),
    cell: ({ row }) =>
      h("input", {
        type: "checkbox",
        checked: row.getIsSelected(),
        onChange: row.getToggleSelectedHandler(),
        class: "h-3.5 w-3.5 rounded border-slate-300",
      }),
  }),
  columnHelper.accessor("pkey", { header: "ID" }),
  ...props.columns.map((col) => columnHelper.accessor((row) => row[col], { id: col, header: col })),
]);

const table = useVueTable({
  get data() {
    return rowsData.value;
  },
  get columns() {
    return columnDefs.value;
  },
  getRowId: (row) => row.pkey,
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  enableRowSelection: true,
  state: {
    get sorting() {
      return sorting.value;
    },
    get rowSelection() {
      return rowSelection.value;
    },
  },
  onSortingChange: (updater: Updater<SortingState>) => {
    sorting.value = typeof updater === "function" ? updater(sorting.value) : updater;
  },
  onRowSelectionChange: (updater: Updater<RowSelectionState>) => {
    const next = typeof updater === "function" ? updater(rowSelection.value) : updater;
    for (const row of rowsData.value) {
      if (rowSelection.value[row.pkey] && !next[row.pkey]) emit("deselect", row.pkey);
    }
    rowSelection.value = next;
  },
});
</script>

<template>
  <aside class="flex h-full w-80 shrink-0 flex-col overflow-hidden rounded-lg border border-slate-200 bg-white">
    <div class="flex items-center justify-between border-b border-slate-200 px-3 py-2.5">
      <h2 class="text-sm font-medium text-slate-700">Sélection ({{ selectedPkeys.length }})</h2>
      <button
        v-if="selectedPkeys.length > 0"
        class="text-xs font-medium text-slate-400 hover:text-red-600"
        @click="emit('clear')"
      >
        Vider
      </button>
    </div>

    <p v-if="selectedPkeys.length === 0" class="px-3 py-6 text-center text-xs text-slate-400">
      Cliquez sur une feature de la carte pour l'ajouter à la sélection.
    </p>
    <p v-else-if="error" class="px-3 py-4 text-xs text-red-600">{{ error }}</p>

    <div v-else class="flex-1 overflow-auto">
      <table class="w-full text-xs">
        <thead class="sticky top-0 border-b border-slate-200 bg-slate-50 text-left font-medium uppercase tracking-wide text-slate-500">
          <tr v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
            <th
              v-for="header in headerGroup.headers"
              :key="header.id"
              class="whitespace-nowrap px-3 py-2"
              :class="{ 'cursor-pointer select-none hover:text-slate-700': header.column.getCanSort() }"
              @click="header.column.getToggleSortingHandler()?.($event)"
            >
              <span class="inline-flex items-center gap-1">
                <FlexRender :render="header.column.columnDef.header" :props="header.getContext()" />
                <span v-if="header.column.getCanSort()" class="text-[10px] text-slate-400">
                  {{ header.column.getIsSorted() === "asc" ? "▲" : header.column.getIsSorted() === "desc" ? "▼" : "" }}
                </span>
              </span>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="row in table.getRowModel().rows" :key="row.id" class="hover:bg-slate-50">
            <td v-for="cell in row.getVisibleCells()" :key="cell.id" class="max-w-[9rem] truncate px-3 py-2 text-slate-700">
              <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="loading" class="px-3 py-2 text-xs text-slate-400">Chargement…</p>
    </div>
  </aside>
</template>
