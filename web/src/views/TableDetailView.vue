<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { deleteTable, exportUrl, getSchema, getTable, listRows } from "../client/tables";
import type { ColumnSchema, Metadata, Row } from "../client/types";
import { ApiError } from "../client/http";

// MapLibre GL is the largest dependency in the bundle — load it only when
// a geo table's map tab is actually opened.
const MapView = defineAsyncComponent(() => import("../components/MapView.vue"));

const props = defineProps<{ tableId: string }>();
const router = useRouter();

const table = ref<Metadata | null>(null);
const columns = ref<ColumnSchema[]>([]);
const rows = ref<Row[]>([]);
const total = ref(0);
const offset = ref(0);
const limit = ref(20);
const tab = ref<"rows" | "schema" | "map">("rows");
const loading = ref(false);
const error = ref<string | null>(null);
const deleting = ref(false);

const rowColumns = computed(() => columns.value.map((c) => c.name).filter((name) => name !== table.value?.geometryColumn));

// The physical column is named "__pkey", but Row objects key it as "pkey"
// (see router.rowToJSON on the backend, which pulls it out of storage.Row's
// dedicated Pkey field rather than its Values map) — map the two back
// together for display rather than showing a blank cell.
function cellValue(row: Row, col: string): Row[string] {
  return col === "__pkey" ? row.pkey : row[col];
}

async function loadTable() {
  table.value = await getTable(props.tableId);
  if (table.value.isGeo) tab.value = "map";
}

async function loadSchema() {
  const res = await getSchema(props.tableId);
  columns.value = res.columns;
}

async function loadRows() {
  const page = await listRows(props.tableId, offset.value, limit.value);
  rows.value = page.rows;
  total.value = page.total;
}

async function loadAll() {
  loading.value = true;
  error.value = null;
  try {
    await Promise.all([loadTable(), loadSchema(), loadRows()]);
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "Impossible de charger la table";
  } finally {
    loading.value = false;
  }
}

async function onDelete() {
  if (!confirm(`Supprimer la table "${table.value?.tableName}" et ses données ?`)) return;
  deleting.value = true;
  try {
    await deleteTable(props.tableId);
    router.push("/tables");
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "Impossible de supprimer la table";
    deleting.value = false;
  }
}

function nextPage() {
  if (offset.value + limit.value >= total.value) return;
  offset.value += limit.value;
}

function prevPage() {
  offset.value = Math.max(0, offset.value - limit.value);
}

watch(offset, loadRows);
onMounted(loadAll);
</script>

<template>
  <div class="mx-auto max-w-5xl px-8 py-8">
    <RouterLink to="/tables" class="text-sm font-medium text-slate-500 hover:text-slate-700">&larr; Tables</RouterLink>

    <div v-if="table" class="mt-2 mb-6 flex items-start justify-between">
      <div>
        <h1 class="text-lg font-semibold text-slate-900">{{ table.tableName }}</h1>
        <p class="text-sm text-slate-500">
          {{ table.rowCount.toLocaleString("fr-FR") }} lignes · mis à jour {{ new Date(table.updatedAt).toLocaleString("fr-FR") }}
        </p>
      </div>
      <div class="flex gap-2">
        <a
          :href="exportUrl(table.tableId, table.isGeo ? 'geojson' : 'csv')"
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        >
          Exporter
        </a>
        <button
          class="rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
          :disabled="deleting"
          @click="onDelete"
        >
          Supprimer
        </button>
      </div>
    </div>

    <p v-if="error" class="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
      {{ error }}
    </p>

    <div class="mb-4 flex gap-1 border-b border-slate-200">
      <button
        v-if="table?.isGeo"
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'map' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'map'"
      >
        Carte
      </button>
      <button
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'rows' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'rows'"
      >
        Données
      </button>
      <button
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'schema' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'schema'"
      >
        Schéma
      </button>
    </div>

    <div v-if="tab === 'map' && table?.isGeo" class="h-[65vh] overflow-hidden rounded-lg border border-slate-200">
      <MapView :table-id="table.tableId" :bbox="table.bbox" />
    </div>

    <div v-else-if="tab === 'schema'" class="overflow-hidden rounded-lg border border-slate-200 bg-white">
      <table class="w-full text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
          <tr>
            <th class="px-4 py-2.5">Colonne</th>
            <th class="px-4 py-2.5">Type</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="col in columns" :key="col.name">
            <td class="px-4 py-2.5 font-medium text-slate-800">{{ col.name }}</td>
            <td class="px-4 py-2.5 font-mono text-xs text-slate-500">{{ col.type }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else>
      <div class="overflow-x-auto rounded-lg border border-slate-200 bg-white">
        <table class="w-full text-sm">
          <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
            <tr>
              <th v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2.5">{{ col }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="row in rows" :key="row.pkey">
              <td v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2.5 text-slate-700">
                {{ cellValue(row, col) }}
              </td>
            </tr>
            <tr v-if="!loading && rows.length === 0">
              <td :colspan="rowColumns.length || 1" class="px-4 py-8 text-center text-sm text-slate-400">Aucune ligne.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="mt-3 flex items-center justify-between text-sm text-slate-500">
        <span>{{ offset + 1 }}–{{ Math.min(offset + limit, total) }} sur {{ total }}</span>
        <div class="flex gap-2">
          <button
            class="rounded-md border border-slate-300 bg-white px-3 py-1 font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
            :disabled="offset === 0"
            @click="prevPage"
          >
            Précédent
          </button>
          <button
            class="rounded-md border border-slate-300 bg-white px-3 py-1 font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
            :disabled="offset + limit >= total"
            @click="nextPage"
          >
            Suivant
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
