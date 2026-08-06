<script setup lang="ts">
import { computed, h, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { Row } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, getFilteredRowModel, useVueTable } from "@tanstack/vue-table";
import { listTables, renameTable } from "../client/tables";
import type { Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import { usePaginatedList } from "../composables/usePaginatedList";
import { useDocumentTitle } from "../composables/useDocumentTitle";
import { parseSorting, stringifySorting } from "../utils/sort";
import { queryString } from "../utils/routeQuery";
import Badge from "../components/ui/Badge.vue";
import DataTable from "../components/ui/DataTable.vue";
import ErrorState from "../components/ui/ErrorState.vue";
import Skeleton from "../components/ui/Skeleton.vue";
import BaseInput from "../components/ui/BaseInput.vue";
import Breadcrumbs from "../components/ui/Breadcrumbs.vue";
import TableNameCell from "../components/tables/TableNameCell.vue";

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

const renamingId = ref<string | null>(null);
const renameError = ref<{ tableId: string; message: string } | null>(null);

// Patches the renamed row in place instead of reloading the whole list: a
// rename from this page must not lose the operator's current scroll
// position, page or search (§8.6).
async function onRename(tableId: string, name: string) {
  renamingId.value = tableId;
  renameError.value = null;
  try {
    const renamed = await renameTable(tableId, name);
    const index = tables.value.findIndex((t) => t.tableId === renamed.tableId);
    if (index !== -1) tables.value[index] = renamed;
  } catch (e) {
    renameError.value = { tableId, message: errorMessage(e, "Impossible de renommer la table") };
  } finally {
    renamingId.value = null;
  }
}

const columnHelper = createColumnHelper<Metadata>();
const columnDefs = [
  columnHelper.accessor("tableName", {
    header: "Nom",
    cell: ({ row }) =>
      h(TableNameCell, {
        tableId: row.original.tableId,
        tableName: row.original.tableName,
        renaming: renamingId.value === row.original.tableId,
        error: renameError.value?.tableId === row.original.tableId ? renameError.value.message : undefined,
        onRename,
      }),
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
  <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
    <Breadcrumbs :items="[{ label: 'Tables disponibles' }]" />
    <div class="flex shrink-0 items-center justify-between">
      <BaseInput v-model="globalFilter" placeholder="Rechercher une table…" class="w-56" />
    </div>

    <Skeleton v-if="loading && !loadedOnce" />
    <ErrorState v-else-if="error" :message="error" @retry="retry" />
    <DataTable
      v-else
      :table="table"
      :row-class="() => 'cursor-pointer hover'"
      :loading="loading"
      :empty-message="tables.length === 0 ? 'Aucune table. Chargez une source depuis l\'onglet Sources.' : 'Aucun résultat.'"
      @row-click="onRowClick"
    >
      <template #footer>
        <div :ref="list.sentinel" class="h-px"></div>
        <p v-if="loading" class="px-4 py-3 text-center text-xs text-base-content/40">Chargement…</p>
      </template>
    </DataTable>
  </div>
</template>
