<script setup lang="ts">
import { computed, h, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import type { Row } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, getFilteredRowModel, useVueTable } from "@tanstack/vue-table";
import { listTables } from "../client/tables";
import type { Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import { usePaginatedList } from "../composables/usePaginatedList";
import { useDocumentTitle } from "../composables/useDocumentTitle";
import { parseSorting, stringifySorting } from "../utils/sort";
import { queryString } from "../utils/routeQuery";
import Badge from "../components/Badge.vue";
import DataTable from "../components/DataTable.vue";
import ErrorState from "../components/ui/ErrorState.vue";
import Skeleton from "../components/ui/Skeleton.vue";

const router = useRouter();
const route = useRoute();

useDocumentTitle(computed(() => "Tables"));

const list = usePaginatedList<Metadata>({
  initialSorting: queryString(route.query.sort) ? parseSorting(queryString(route.query.sort)) : [{ id: "updatedAt", desc: true }],
  fetchPage: (offset, limit, sort) => listTables(offset, limit, sort).then((page) => ({ items: page.tables, total: page.total })),
  describeError: (e) => errorMessage(e, "Impossible de charger les tables"),
});
const { items: tables, total, loading, loadedOnce, error, sorting, setSorting, loadMore, retry } = list;
const globalFilter = ref(queryString(route.query.q));

watch([globalFilter, sorting], ([q, sort]) => {
  router.replace({ query: { ...route.query, q: q.trim() || undefined, sort: stringifySorting(sort) || undefined } });
});

// When search is active, load all remaining pages to filter across the full
// dataset: client-side filtering on partial pages violates §8.6 (a silent
// partial result is a lie).
watch(globalFilter, async (filter) => {
  if (filter.trim() && tables.value.length < total.value) {
    await loadMore();
  }
});

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
    cell: ({ row }) => h(Badge, { tone: row.original.isGeo ? "primary" : "neutral" }, () => (row.original.isGeo ? "Géo" : "Standard")),
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
  <div class="flex min-h-0 flex-1 flex-col px-6 py-4">
    <div class="mb-4 flex shrink-0 items-center justify-between">
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

    <Skeleton v-if="loading && !loadedOnce" />
    <ErrorState v-else-if="error" :message="error" @retry="retry" />
    <DataTable
      v-else
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
