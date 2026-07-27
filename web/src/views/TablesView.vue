<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import { listTables } from "../client/tables";
import type { Metadata } from "../client/types";
import { ApiError } from "../client/http";

const tables = ref<Metadata[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    tables.value = (await listTables()) ?? [];
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "Impossible de charger les tables";
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="mx-auto max-w-4xl px-8 py-8">
    <div class="mb-6">
      <h1 class="text-lg font-semibold text-slate-900">Tables</h1>
      <p class="text-sm text-slate-500">Tables chargées ou dérivées dans le serveur.</p>
    </div>

    <p v-if="error" class="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
      {{ error }}
    </p>

    <div class="overflow-hidden rounded-lg border border-slate-200 bg-white">
      <table class="w-full text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
          <tr>
            <th class="px-4 py-2.5">Nom</th>
            <th class="px-4 py-2.5">Type</th>
            <th class="px-4 py-2.5">Lignes</th>
            <th class="px-4 py-2.5">Mise à jour</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr
            v-for="table in tables"
            :key="table.tableId"
            class="cursor-pointer hover:bg-slate-50"
            @click="$router.push(`/tables/${table.tableId}`)"
          >
            <td class="px-4 py-2.5">
              <RouterLink :to="`/tables/${table.tableId}`" class="font-medium text-indigo-600 hover:text-indigo-700">
                {{ table.tableName }}
              </RouterLink>
            </td>
            <td class="px-4 py-2.5">
              <span
                v-if="table.isGeo"
                class="inline-flex items-center rounded-full bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-700"
              >
                Géo
              </span>
              <span v-else class="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600">
                Standard
              </span>
            </td>
            <td class="px-4 py-2.5 text-slate-600">{{ table.rowCount.toLocaleString("fr-FR") }}</td>
            <td class="px-4 py-2.5 text-slate-500">{{ new Date(table.updatedAt).toLocaleString("fr-FR") }}</td>
          </tr>
          <tr v-if="!loading && tables.length === 0">
            <td colspan="4" class="px-4 py-8 text-center text-sm text-slate-400">
              Aucune table. Chargez une source depuis l'onglet Sources.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
