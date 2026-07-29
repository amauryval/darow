<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { deleteSource, getSourceMetadata, listSources, loadSource, refreshSources, uploadSource } from "../client/sources";
import type { Source } from "../client/types";
import { errorMessage } from "../client/http";
import { UploadCloud } from "@lucide/vue";
import Badge from "../components/Badge.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import { useToast } from "../composables/useToast";

const router = useRouter();
const toast = useToast();

const sources = ref<Source[]>([]);
const loading = ref(false);
const loadingName = ref<string | null>(null);
const deletingName = ref<string | null>(null);
const uploading = ref(false);
const dragging = ref(false);
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
  try {
    sources.value = (await listSources()) ?? [];
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les sources"));
  } finally {
    loading.value = false;
  }
}

async function refresh() {
  loading.value = true;
  try {
    sources.value = (await refreshSources()) ?? [];
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de rafraîchir les sources"));
  } finally {
    loading.value = false;
  }
}

async function startLoad(source: Source, layerName?: string) {
  const key = layerName ? layerKey(source, layerName) : source.name;
  epsgEditingKey.value = key;
  epsgInput.value = "";
  epsgDetecting.value = true;
  try {
    const crs = await getSourceMetadata(source.name, layerName);
    if (epsgEditingKey.value === key) {
      epsgInput.value = crs.srid ? String(crs.srid) : "";
    }
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de détecter le système de coordonnées de ${key}`));
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
  try {
    await loadSource(source.name, layerName, Number(epsgInput.value));
    epsgEditingKey.value = null;
    // refresh (not load): the backend's source cache only recomputes each
    // entry's `loaded` status on an explicit rescan (LoadedSourceNames is
    // read inside Refresh) — Load() itself never touches that cache, so a
    // plain re-fetch of the stale cache would still show "Non chargée".
    await refresh();
    toast.success(`${key} chargé.`);
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de charger ${key}`));
  } finally {
    loadingName.value = null;
  }
}

async function onDelete(name: string) {
  deletingName.value = name;
  try {
    await deleteSource(name);
    await refresh();
    toast.success(`${name} supprimé.`);
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de supprimer ${name}`));
  } finally {
    deletingName.value = null;
  }
}

function triggerUpload() {
  fileInput.value?.click();
}

async function handleUpload(file: File) {
  uploading.value = true;
  try {
    const result = await uploadSource(file);
    if ("tableId" in result) {
      toast.success(`${file.name} chargé.`);
      router.push(`/tables/${result.tableId}`);
    } else {
      // Multi-layer GeoPackage: no layer was auto-selected — show it in the
      // list so the user can load a specific layer.
      await refresh();
      toast.success(`${file.name} ajouté — choisissez une couche à charger.`);
    }
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de charger ${file.name}`));
  } finally {
    uploading.value = false;
  }
}

async function onFileSelected(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (fileInput.value) fileInput.value.value = "";
  if (!file) return;
  await handleUpload(file);
}

async function onDrop(event: DragEvent) {
  dragging.value = false;
  const file = event.dataTransfer?.files?.[0];
  if (!file) return;
  await handleUpload(file);
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
      <button
        class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
        :disabled="loading"
        @click="refresh"
      >
        Rafraîchir
      </button>
    </div>

    <div
      class="mb-6 flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors"
      :class="dragging ? 'border-indigo-400 bg-indigo-50' : 'border-slate-200 bg-white hover:border-slate-300'"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <UploadCloud :size="32" class="text-slate-400" />
      <p class="text-sm text-slate-600">Glissez un fichier ici, ou</p>
      <input ref="fileInput" type="file" class="hidden" @change="onFileSelected" />
      <button
        class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
        :disabled="uploading"
        @click="triggerUpload"
      >
        {{ uploading ? "Envoi…" : "Parcourir" }}
      </button>
    </div>

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
                <Badge :tone="source.loaded ? 'emerald' : 'slate'">{{ source.loaded ? "Chargée" : "Non chargée" }}</Badge>
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
                  <ConfirmButton
                    label="Supprimer"
                    :busy="deletingName === source.name"
                    @confirm="onDelete(source.name)"
                  />
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
                  <ConfirmButton
                    label="Supprimer le fichier"
                    :busy="deletingName === source.name"
                    @confirm="onDelete(source.name)"
                  />
                </td>
              </tr>
              <tr v-for="layer in source.layers" :key="layerKey(source, layer.name)">
                <td class="px-4 py-2.5 pl-8 text-slate-700">{{ layer.name }}</td>
                <td class="px-4 py-2.5"></td>
                <td class="px-4 py-2.5">
                  <Badge :tone="layer.loaded ? 'emerald' : 'slate'">{{ layer.loaded ? "Chargée" : "Non chargée" }}</Badge>
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
