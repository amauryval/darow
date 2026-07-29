<script setup lang="ts">
import { h, onMounted, onUnmounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import type { Row, SortingState, Updater } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, getFilteredRowModel, useVueTable } from "@tanstack/vue-table";
import { listTables } from "../client/tables";
import type { Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import { sortingToQuery } from "../utils/sort";
import Badge from "../components/Badge.vue";
import DataTable from "../components/DataTable.vue";
import { useToast } from "../composables/useToast";

const router = useRouter();
const toast = useToast();

const limit = 50;
const tables = ref<Metadata[]>([]);
const total = ref(0);
const offset = ref(0);
const loadingMore = ref(false);
const loadedOnce = ref(false);
const sorting = ref<SortingState>([{ id: "updatedAt", desc: true }]);
const globalFilter = ref("");

async function loadMore() {
  if (loadingMore.value || (loadedOnce.value && tables.value.length >= total.value)) return;
  loadingMore.value = true;
  try {
    const page = await listTables(offset.value, limit, sortingToQuery(sorting.value));
    tables.value = [...tables.value, ...page.tables];
    total.value = page.total;
    offset.value += page.tables.length;
    loadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les tables"));
  } finally {
    loadingMore.value = false;
  }
}

// Sorting is server-side (offset/limit pagination can't be sorted
// correctly client-side once only part of the data is loaded) — a change
// invalidates every already-loaded page, so it resets to page 1 like any
// other mutation. Fetches page 1 before touching tables.value, so the
// table keeps showing its current rows (rather than flashing empty) until
// the new page is ready to swap in.
async function reload() {
  loadingMore.value = true;
  try {
    const page = await listTables(0, limit, sortingToQuery(sorting.value));
    tables.value = page.tables;
    total.value = page.total;
    offset.value = page.tables.length;
    loadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les tables"));
  } finally {
    loadingMore.value = false;
  }
}

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
  onSortingChange: (updater: Updater<SortingState>) => {
    sorting.value = typeof updater === "function" ? updater(sorting.value) : updater;
    reload();
  },
  onGlobalFilterChange: (updater: Updater<string>) => {
    globalFilter.value = typeof updater === "function" ? updater(globalFilter.value) : updater;
  },
});

const sentinel = ref<HTMLElement | null>(null);
let observer: IntersectionObserver | null = null;

onMounted(() => {
  observer = new IntersectionObserver((entries) => {
    if (entries[0]?.isIntersecting) loadMore();
  }, { rootMargin: "200px" });
  if (sentinel.value) observer.observe(sentinel.value);
  loadMore();
});

onUnmounted(() => observer?.disconnect());
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
      :loading="loadingMore"
      :empty-message="tables.length === 0 ? 'Aucune table. Chargez une source depuis l\'onglet Sources.' : 'Aucun résultat.'"
      @row-click="onRowClick"
    >
      <template #footer>
        <div ref="sentinel" class="h-px"></div>
        <p v-if="loadingMore" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
      </template>
    </DataTable>
  </div>
</template>
