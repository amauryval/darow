<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { listSources, loadSource, refreshSources, uploadSource } from "../client/sources";
import type { Source } from "../client/types";
import { ApiError } from "../client/http";

const router = useRouter();

const sources = ref<Source[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);
const loadingName = ref<string | null>(null);
const uploading = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    sources.value = (await listSources()) ?? [];
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "Impossible de charger les sources";
  } finally {
    loading.value = false;
  }
}

async function refresh() {
  loading.value = true;
  error.value = null;
  try {
    sources.value = (await refreshSources()) ?? [];
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "Impossible de rafraîchir les sources";
  } finally {
    loading.value = false;
  }
}

async function onLoad(source: Source) {
  loadingName.value = source.name;
  error.value = null;
  try {
    await loadSource(source.name);
    // refresh (not load): the backend's source cache only recomputes each
    // entry's `loaded` status on an explicit rescan (LoadedSourceNames is
    // read inside Refresh) — Load() itself never touches that cache, so a
    // plain re-fetch of the stale cache would still show "Non chargée".
    await refresh();
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : `Impossible de charger ${source.name}`;
  } finally {
    loadingName.value = null;
  }
}

function triggerUpload() {
  fileInput.value?.click();
}

async function onFileSelected(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;

  uploading.value = true;
  error.value = null;
  try {
    const { tableId } = await uploadSource(file);
    router.push(`/tables/${tableId}`);
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : `Impossible de charger ${file.name}`;
  } finally {
    uploading.value = false;
    if (fileInput.value) fileInput.value.value = "";
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} Ko`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} Mo`;
}

onMounted(load);
</script>

<template>
  <div class="mx-auto max-w-4xl px-8 py-8">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-lg font-semibold text-slate-900">Sources</h1>
        <p class="text-sm text-slate-500">Fichiers détectés dans le répertoire de travail du serveur.</p>
      </div>
      <div class="flex gap-2">
        <button
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          :disabled="loading"
          @click="refresh"
        >
          Rafraîchir
        </button>
        <input ref="fileInput" type="file" class="hidden" @change="onFileSelected" />
        <button
          class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
          :disabled="uploading"
          @click="triggerUpload"
        >
          {{ uploading ? "Envoi..." : "Ajouter un fichier" }}
        </button>
      </div>
    </div>

    <p v-if="error" class="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
      {{ error }}
    </p>

    <div class="overflow-hidden rounded-lg border border-slate-200 bg-white">
      <table class="w-full text-sm">
        <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
          <tr>
            <th class="px-4 py-2.5">Nom</th>
            <th class="px-4 py-2.5">Taille</th>
            <th class="px-4 py-2.5">Statut</th>
            <th class="px-4 py-2.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="source in sources" :key="source.name">
            <td class="px-4 py-2.5 font-medium text-slate-800">{{ source.name }}</td>
            <td class="px-4 py-2.5 text-slate-500">{{ formatSize(source.sizeBytes) }}</td>
            <td class="px-4 py-2.5">
              <span
                v-if="source.loaded"
                class="inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700"
              >
                Chargée
              </span>
              <span v-else class="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600">
                Non chargée
              </span>
            </td>
            <td class="px-4 py-2.5 text-right">
              <RouterLink
                v-if="source.loaded && source.tableId"
                :to="`/tables/${source.tableId}`"
                class="text-sm font-medium text-indigo-600 hover:text-indigo-700"
              >
                Voir la table
              </RouterLink>
              <button
                v-else
                class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
                :disabled="loadingName === source.name"
                @click="onLoad(source)"
              >
                {{ loadingName === source.name ? "Chargement..." : "Charger" }}
              </button>
            </td>
          </tr>
          <tr v-if="!loading && sources.length === 0">
            <td colspan="4" class="px-4 py-8 text-center text-sm text-slate-400">Aucune source détectée.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
