<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { listTables } from "../client/tables";
import type { Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import Badge from "../components/Badge.vue";
import { useToast } from "../composables/useToast";

const router = useRouter();
const toast = useToast();

const tables = ref<Metadata[]>([]);
const loading = ref(false);
const search = ref("");

type SortKey = "tableName" | "rowCount" | "updatedAt";
const sortKey = ref<SortKey>("updatedAt");
const sortDesc = ref(true);

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  const rows = q ? tables.value.filter((t) => t.tableName.toLowerCase().includes(q)) : tables.value.slice();
  rows.sort((a, b) => {
    const cmp =
      sortKey.value === "tableName"
        ? a.tableName.localeCompare(b.tableName)
        : sortKey.value === "rowCount"
          ? a.rowCount - b.rowCount
          : a.updatedAt.localeCompare(b.updatedAt);
    return sortDesc.value ? -cmp : cmp;
  });
  return rows;
});

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDesc.value = !sortDesc.value;
  } else {
    sortKey.value = key;
    sortDesc.value = key !== "tableName";
  }
}

async function load() {
  loading.value = true;
  try {
    tables.value = (await listTables()) ?? [];
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les tables"));
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="mx-auto max-w-5xl px-8 py-8">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-lg font-semibold text-slate-900">Tables</h1>
        <p class="text-sm text-slate-500">Tables chargées ou dérivées dans le serveur.</p>
      </div>
      <input
        v-model="search"
        type="text"
        placeholder="Rechercher une table…"
        class="w-56 rounded-md border border-slate-300 px-3 py-1.5 text-sm placeholder:text-slate-400 focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
    </div>

    <div class="overflow-hidden rounded-lg border border-slate-200 bg-white">
      <table class="w-full text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
          <tr>
            <th class="cursor-pointer select-none px-4 py-2.5 hover:text-slate-700" @click="toggleSort('tableName')">
              Nom <span v-if="sortKey === 'tableName'">{{ sortDesc ? "↓" : "↑" }}</span>
            </th>
            <th class="px-4 py-2.5">Type</th>
            <th class="cursor-pointer select-none px-4 py-2.5 hover:text-slate-700" @click="toggleSort('rowCount')">
              Lignes <span v-if="sortKey === 'rowCount'">{{ sortDesc ? "↓" : "↑" }}</span>
            </th>
            <th class="cursor-pointer select-none px-4 py-2.5 hover:text-slate-700" @click="toggleSort('updatedAt')">
              Mise à jour <span v-if="sortKey === 'updatedAt'">{{ sortDesc ? "↓" : "↑" }}</span>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr
            v-for="table in filtered"
            :key="table.tableId"
            class="cursor-pointer transition-colors hover:bg-slate-50"
            @click="router.push(`/tables/${table.tableId}`)"
          >
            <td class="px-4 py-2.5">
              <RouterLink :to="`/tables/${table.tableId}`" class="font-medium text-indigo-600 hover:text-indigo-700">
                {{ table.tableName }}
              </RouterLink>
            </td>
            <td class="px-4 py-2.5">
              <Badge :tone="table.isGeo ? 'indigo' : 'slate'">{{ table.isGeo ? "Géo" : "Standard" }}</Badge>
            </td>
            <td class="px-4 py-2.5 text-slate-600">{{ table.rowCount.toLocaleString("fr-FR") }}</td>
            <td class="px-4 py-2.5 text-slate-500">{{ new Date(table.updatedAt).toLocaleString("fr-FR") }}</td>
          </tr>
          <tr v-if="!loading && filtered.length === 0">
            <td colspan="4" class="px-4 py-8 text-center text-sm text-slate-400">
              {{ tables.length === 0 ? "Aucune table. Chargez une source depuis l'onglet Sources." : "Aucun résultat." }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
