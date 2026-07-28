<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { deleteSource, getSourceMetadata, listSources, loadSource, refreshSources, uploadSource } from "../client/sources";
import type { Source } from "../client/types";
import { errorMessage } from "../client/http";

const router = useRouter();

const sources = ref<Source[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);
const loadingName = ref<string | null>(null);
const deletingName = ref<string | null>(null);
const uploading = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

// EPSG-editing state for the row/layer currently being loaded: null when
// no row is mid-load. epsgInput is the editable value shown to the user,
// pre-filled from getSourceMetadata's detection but always overridable —
// required outright when detection comes back null.
const epsgEditingKey = ref<string | null>(null);
const epsgInput = ref("");
const epsgDetecting = ref(false);

function layerKey(source: Source, layerName: string): string {
  return `${source.name}::${layerName}`;
}

function isValidEpsg(value: string): boolean {
  return /^[1-9]\d*$/.test(value.trim());
}

async function load() {
  loading.value = true;
  error.value = null;
  try {
    sources.value = (await listSources()) ?? [];
  } catch (e) {
    error.value = errorMessage(e, "Impossible de charger les sources");
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
    error.value = errorMessage(e, "Impossible de rafraîchir les sources");
  } finally {
    loading.value = false;
  }
}

async function startLoad(source: Source, layerName?: string) {
  const key = layerName ? layerKey(source, layerName) : source.name;
  epsgEditingKey.value = key;
  epsgInput.value = "";
  epsgDetecting.value = true;
  error.value = null;
  try {
    const crs = await getSourceMetadata(source.name, layerName);
    if (epsgEditingKey.value === key) {
      epsgInput.value = crs.srid ? String(crs.srid) : "";
    }
  } catch (e) {
    error.value = errorMessage(e, `Impossible de détecter le système de coordonnées de ${key}`);
  } finally {
    epsgDetecting.value = false;
  }
}

function cancelLoad() {
  epsgEditingKey.value = null;
  epsgInput.value = "";
}

async function confirmLoad(source: Source, layerName?: string) {
  const key = layerName ? layerKey(source, layerName) : source.name;
  loadingName.value = key;
  error.value = null;
  try {
    await loadSource(source.name, layerName, Number(epsgInput.value));
    epsgEditingKey.value = null;
    // refresh (not load): the backend's source cache only recomputes each
    // entry's `loaded` status on an explicit rescan (LoadedSourceNames is
    // read inside Refresh) — Load() itself never touches that cache, so a
    // plain re-fetch of the stale cache would still show "Non chargée".
    await refresh();
  } catch (e) {
    error.value = errorMessage(e, `Impossible de charger ${key}`);
  } finally {
    loadingName.value = null;
  }
}

async function onDelete(name: string) {
  if (!confirm(`Supprimer le fichier "${name}" ? La table associée sera aussi supprimée si elle existe.`)) return;
  deletingName.value = name;
  error.value = null;
  try {
    await deleteSource(name);
    await refresh();
  } catch (e) {
    error.value = errorMessage(e, `Impossible de supprimer ${name}`);
  } finally {
    deletingName.value = null;
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
    const result = await uploadSource(file);
    if ("tableId" in result) {
      router.push(`/tables/${result.tableId}`);
    } else {
      // Multi-layer GeoPackage: no layer was auto-selected — show it in the
      // list so the user can load a specific layer.
      await refresh();
    }
  } catch (e) {
    error.value = errorMessage(e, `Impossible de charger ${file.name}`);
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
          <template v-for="source in sources" :key="source.name">
            <tr v-if="!source.layers || source.layers.length === 0">
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
                <div v-else-if="epsgEditingKey === source.name" class="flex items-center justify-end gap-2">
                  <input
                    v-model="epsgInput"
                    type="text"
                    inputmode="numeric"
                    :placeholder="epsgDetecting ? 'Détection...' : 'Code EPSG'"
                    :disabled="epsgDetecting"
                    class="w-24 rounded-md border border-slate-300 px-2 py-1 text-xs disabled:bg-slate-50"
                  />
                  <button
                    class="rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                    :disabled="epsgDetecting || !isValidEpsg(epsgInput) || loadingName === source.name"
                    @click="confirmLoad(source)"
                  >
                    {{ loadingName === source.name ? "Chargement..." : "Confirmer" }}
                  </button>
                  <button
                    class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                    @click="cancelLoad"
                  >
                    Annuler
                  </button>
                </div>
                <div v-else class="flex items-center justify-end gap-2">
                  <button
                    class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                    @click="startLoad(source)"
                  >
                    Charger
                  </button>
                  <button
                    class="rounded-md border border-red-200 bg-white px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
                    :disabled="deletingName === source.name"
                    @click="onDelete(source.name)"
                  >
                    {{ deletingName === source.name ? "Suppression..." : "Supprimer" }}
                  </button>
                </div>
              </td>
            </tr>
            <template v-else>
              <tr>
                <td class="px-4 py-2.5 font-medium text-slate-800" colspan="2">{{ source.name }}</td>
                <td class="px-4 py-2.5 text-xs text-slate-400" colspan="1">
                  {{ source.layers.length }} couches — {{ formatSize(source.sizeBytes) }}
                </td>
                <td class="px-4 py-2.5 text-right">
                  <button
                    class="rounded-md border border-red-200 bg-white px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
                    :disabled="deletingName === source.name"
                    @click="onDelete(source.name)"
                  >
                    {{ deletingName === source.name ? "Suppression..." : "Supprimer le fichier" }}
                  </button>
                </td>
              </tr>
              <tr v-for="layer in source.layers" :key="layerKey(source, layer.name)">
                <td class="px-4 py-2.5 pl-8 text-slate-700">{{ layer.name }}</td>
                <td class="px-4 py-2.5"></td>
                <td class="px-4 py-2.5">
                  <span
                    v-if="layer.loaded"
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
                    v-if="layer.loaded && layer.tableId"
                    :to="`/tables/${layer.tableId}`"
                    class="text-sm font-medium text-indigo-600 hover:text-indigo-700"
                  >
                    Voir la table
                  </RouterLink>
                  <div v-else-if="epsgEditingKey === layerKey(source, layer.name)" class="flex items-center justify-end gap-2">
                    <input
                      v-model="epsgInput"
                      type="text"
                      inputmode="numeric"
                      :placeholder="epsgDetecting ? 'Détection...' : 'Code EPSG'"
                      :disabled="epsgDetecting"
                      class="w-24 rounded-md border border-slate-300 px-2 py-1 text-xs disabled:bg-slate-50"
                    />
                    <button
                      class="rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                      :disabled="epsgDetecting || !isValidEpsg(epsgInput) || loadingName === layerKey(source, layer.name)"
                      @click="confirmLoad(source, layer.name)"
                    >
                      {{ loadingName === layerKey(source, layer.name) ? "Chargement..." : "Confirmer" }}
                    </button>
                    <button
                      class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                      @click="cancelLoad"
                    >
                      Annuler
                    </button>
                  </div>
                  <button
                    v-else
                    class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                    @click="startLoad(source, layer.name)"
                  >
                    Charger
                  </button>
                </td>
              </tr>
            </template>
          </template>
          <tr v-if="!loading && sources.length === 0">
            <td colspan="4" class="px-4 py-8 text-center text-sm text-slate-400">Aucune source détectée.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
