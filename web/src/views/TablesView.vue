<script setup lang="ts">
import { h, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import type { Row } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, getFilteredRowModel, useVueTable } from "@tanstack/vue-table";
import { listTables } from "../client/tables";
import type { Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import { usePaginatedList } from "../composables/usePaginatedList";
import { useToast } from "../composables/useToast";
import Badge from "../components/Badge.vue";
import DataTable from "../components/DataTable.vue";

const router = useRouter();
const toast = useToast();

// sentinel stays nested under `list` rather than destructured: template
// refs bound via `:ref` need the actual Ref object, but a top-level
// destructured ref auto-unwraps when read inside the template.
const list = usePaginatedList<Metadata>({
  initialSorting: [{ id: "updatedAt", desc: true }],
  fetchPage: (offset, limit, sort) => listTables(offset, limit, sort).then((page) => ({ items: page.tables, total: page.total })),
  onError: (e) => toast.error(errorMessage(e, "Impossible de charger les tables")),
});
const { items: tables, loading, sorting, setSorting } = list;
const globalFilter = ref("");

function onRowClick(row: Row<Metadata>) {
  router.push(`/tables/${row.original.tableId}`);
}

const columnHelper = createColumnHelper<Metadata>();
const columnDefs = [
  columnHelper.accessor("tableName", {
    header: "Nom",
    cell: ({ row }) =>
      h(RouterLink, { to: `/tables/${row.original.tableId}`, class: "font-medium text-indigo-600 hover:text-indigo-700" }, () =>
        row.original.tableName,
      ),
  }),
  columnHelper.display({
    id: "type",
    header: "Type",
    enableGlobalFilter: false,
    enableSorting: false,
    cell: ({ row }) => h(Badge, { tone: row.original.isGeo ? "indigo" : "slate" }, () => (row.original.isGeo ? "Géo" : "Standard")),
  }),
  columnHelper.accessor("rowCount", {
    header: "Lignes",
    enableGlobalFilter: false,
    cell: ({ getValue }) => getValue().toLocaleString("fr-FR"),
  }),
  columnHelper.accessor("updatedAt", {
    header: "Mise à jour",
    enableGlobalFilter: false,
    cell: ({ getValue }) => new Date(getValue()).toLocaleString("fr-FR"),
  }),
];

const table = useVueTable({
  get data() {
    return tables.value;
  },
  columns: columnDefs,
  getCoreRowModel: getCoreRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  manualSorting: true,
  state: {
    get sorting() {
      return sorting.value;
    },
    get globalFilter() {
      return globalFilter.value;
    },
  },
  onSortingChange: setSorting,
  onGlobalFilterChange: (updater) => {
    globalFilter.value = typeof updater === "function" ? updater(globalFilter.value) : updater;
  },
});
</script>

<template>
  <div class="mx-auto flex h-full max-w-5xl flex-col px-8 py-8">
    <div class="mb-6 flex shrink-0 items-center justify-between">
      <div>
        <h1 class="text-lg font-semibold text-slate-900">Tables</h1>
        <p class="text-sm text-slate-500">Tables chargées ou dérivées dans le serveur.</p>
      </div>
      <input
        v-model="globalFilter"
        type="text"
        placeholder="Rechercher une table…"
        class="w-56 rounded-md border border-slate-300 px-3 py-1.5 text-sm placeholder:text-slate-400 focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
    </div>

    <DataTable
      :table="table"
      :row-class="() => 'cursor-pointer transition-colors hover:bg-slate-50'"
      :loading="loading"
      :empty-message="tables.length === 0 ? 'Aucune table. Chargez une source depuis l\'onglet Sources.' : 'Aucun résultat.'"
      @row-click="onRowClick"
    >
      <template #footer>
        <div :ref="list.sentinel" class="h-px"></div>
        <p v-if="loading" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
      </template>
    </DataTable>
  </div>
</template>
