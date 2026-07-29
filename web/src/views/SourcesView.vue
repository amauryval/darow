<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import type { SortingState, Updater } from "@tanstack/vue-table";
import { createColumnHelper, getCoreRowModel, getExpandedRowModel, useVueTable } from "@tanstack/vue-table";
import { deleteSource, getSourceMetadata, listSources, loadSource, refreshSources, uploadSource } from "../client/sources";
import type { Source } from "../client/types";
import { errorMessage } from "../client/http";
import { sortingToQuery } from "../utils/sort";
import { UploadCloud } from "@lucide/vue";
import Badge from "../components/Badge.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import DataTable from "../components/DataTable.vue";
import { useToast } from "../composables/useToast";

// Display column ids that don't match the API's Source field names 1:1
// ("size" formats sizeBytes+layerCount, "status" formats loaded).
const sourceSortColumnMap: Record<string, string> = { size: "sizeBytes", status: "loaded" };

const router = useRouter();
const toast = useToast();

const limit = 50;
const sources = ref<Source[]>([]);
const total = ref(0);
const offset = ref(0);
const loadingMore = ref(false);
const loadedOnce = ref(false);
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
const loadingName = ref<string | null>(null);
const deletingName = ref<string | null>(null);
const sorting = ref<SortingState>([]);

function layerKey(source: Source, layerName: string): string {
  return `${source.name}::${layerName}`;
}

function isValidEpsg(value: string): boolean {
  return /^[1-9]\d*$/.test(value.trim());
}

async function loadMore() {
  if (loadingMore.value || (loadedOnce.value && sources.value.length >= total.value)) return;
  loadingMore.value = true;
  try {
    const page = await listSources(offset.value, limit, sortingToQuery(sorting.value, sourceSortColumnMap));
    sources.value = [...sources.value, ...page.sources];
    total.value = page.total;
    offset.value += page.sources.length;
    loadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les sources"));
  } finally {
    loadingMore.value = false;
  }
}

// A mutating action (load/delete/refresh) can change any already-fetched
// page, and re-fetching only that one page from the server wouldn't
// re-derive the running total correctly either — resetting to page 1 is
// the simplest correct behavior, matching how infinite-scroll lists
// commonly settle after a mutation.
async function reload() {
  sources.value = [];
  total.value = 0;
  offset.value = 0;
  loadedOnce.value = false;
  await loadMore();
}

// Like reload(), but fetches page 1 before touching sources.value, so the
// table keeps showing its current rows instead of flashing empty while a
// sort change's request is in flight.
async function reloadSorted() {
  loadingMore.value = true;
  try {
    const page = await listSources(0, limit, sortingToQuery(sorting.value, sourceSortColumnMap));
    sources.value = page.sources;
    total.value = page.total;
    offset.value = page.sources.length;
    loadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger les sources"));
  } finally {
    loadingMore.value = false;
  }
}

async function refresh() {
  try {
    const page = await refreshSources(0, limit, sortingToQuery(sorting.value, sourceSortColumnMap));
    sources.value = page.sources;
    total.value = page.total;
    offset.value = page.sources.length;
    loadedOnce.value = true;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de rafraîchir les sources"));
  }
}

interface SourceRow {
  name: string;
  sizeBytes?: number;
  loaded: boolean;
  tableId?: string;
  layerCount?: number;
  source: Source;
  layerName?: string;
  subRows?: SourceRow[];
}

const rows = computed<SourceRow[]>(() =>
  sources.value.map((source) => {
    if (source.layers && source.layers.length > 0) {
      return {
        name: source.name,
        loaded: false,
        layerCount: source.layers.length,
        source,
        subRows: source.layers.map((layer) => ({
          name: layer.name,
          loaded: layer.loaded,
          tableId: layer.tableId,
          source,
          layerName: layer.name,
        })),
      };
    }
    return { name: source.name, sizeBytes: source.sizeBytes, loaded: source.loaded, tableId: source.tableId, source };
  }),
);

async function startLoad(row: SourceRow) {
  const key = row.layerName ? layerKey(row.source, row.layerName) : row.source.name;
  epsgEditingKey.value = key;
  epsgInput.value = "";
  epsgDetecting.value = true;
  try {
    const crs = await getSourceMetadata(row.source.name, row.layerName);
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

async function confirmLoad(row: SourceRow) {
  const key = row.layerName ? layerKey(row.source, row.layerName) : row.source.name;
  loadingName.value = key;
  try {
    await loadSource(row.source.name, row.layerName, Number(epsgInput.value));
    epsgEditingKey.value = null;
    await reload();
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
    await reload();
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
      await reload();
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

const columnHelper = createColumnHelper<SourceRow>();
const columnDefs = [
  columnHelper.accessor("name", {
    header: "Nom",
    cell: ({ row, getValue }) =>
      h("span", { class: row.depth > 0 ? "pl-8 text-slate-700" : "font-medium text-slate-800" }, getValue()),
  }),
  columnHelper.display({
    id: "size",
    header: "Taille",
    enableSorting: true,
    cell: ({ row }) =>
      row.original.layerCount !== undefined
        ? h("span", { class: "text-xs text-slate-400" }, `${row.original.layerCount} couches — ${formatSize(row.original.source.sizeBytes)}`)
        : row.original.sizeBytes !== undefined
          ? h("span", { class: "text-slate-500" }, formatSize(row.original.sizeBytes))
          : null,
  }),
  columnHelper.display({
    id: "status",
    header: "Statut",
    enableSorting: true,
    cell: ({ row }) =>
      row.original.layerCount !== undefined ? null : h(Badge, { tone: row.original.loaded ? "emerald" : "slate" }, () => (row.original.loaded ? "Chargée" : "Non chargée")),
  }),
  columnHelper.display({
    id: "actions",
    header: "",
    meta: { align: "right" },
    cell: ({ row }) => {
      const r = row.original;
      if (r.layerCount !== undefined) {
        return h("div", { class: "flex justify-end" }, [
          h(ConfirmButton, {
            label: "Supprimer le fichier",
            busy: deletingName.value === r.source.name,
            onConfirm: () => onDelete(r.source.name),
          }),
        ]);
      }
      if (r.loaded && r.tableId) {
        return h(RouterLink, { to: `/tables/${r.tableId}`, class: "text-sm font-medium text-indigo-600 hover:text-indigo-700" }, () => "Voir la table");
      }
      const key = r.layerName ? layerKey(r.source, r.layerName) : r.source.name;
      if (epsgEditingKey.value === key) {
        return h("div", { class: "flex items-center justify-end gap-2" }, [
          h("input", {
            type: "text",
            inputmode: "numeric",
            value: epsgInput.value,
            placeholder: epsgDetecting.value ? "Détection..." : "Code EPSG",
            disabled: epsgDetecting.value,
            onInput: (e: Event) => (epsgInput.value = (e.target as HTMLInputElement).value),
            class: "w-24 rounded-md border border-slate-300 px-2 py-1 text-xs disabled:bg-slate-50",
          }),
          h(
            "button",
            {
              class: "rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50",
              disabled: epsgDetecting.value || !isValidEpsg(epsgInput.value) || loadingName.value === key,
              onClick: () => confirmLoad(r),
            },
            loadingName.value === key ? "Chargement..." : "Confirmer",
          ),
          h(
            "button",
            {
              class: "rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50",
              onClick: cancelLoad,
            },
            "Annuler",
          ),
        ]);
      }
      if (r.layerName) {
        return h(
          "button",
          {
            class: "rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50",
            onClick: () => startLoad(r),
          },
          "Charger",
        );
      }
      return h("div", { class: "flex items-center justify-end gap-2" }, [
        h(
          "button",
          {
            class: "rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50",
            onClick: () => startLoad(r),
          },
          "Charger",
        ),
        h(ConfirmButton, { label: "Supprimer", busy: deletingName.value === r.source.name, onConfirm: () => onDelete(r.source.name) }),
      ]);
    },
  }),
];

const table = useVueTable({
  get data() {
    return rows.value;
  },
  columns: columnDefs,
  getRowId: (row, _index, parent) => (parent ? `${parent.id}::${row.name}` : row.name),
  getCoreRowModel: getCoreRowModel(),
  getExpandedRowModel: getExpandedRowModel(),
  getSubRows: (row) => row.subRows,
  manualSorting: true,
  state: {
    expanded: true,
    get sorting() {
      return sorting.value;
    },
  },
  onSortingChange: (updater: Updater<SortingState>) => {
    sorting.value = typeof updater === "function" ? updater(sorting.value) : updater;
    reloadSorted();
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
  <div class="mx-auto flex h-full max-w-4xl flex-col px-8 py-8">
    <div class="mb-6 flex shrink-0 items-center justify-between">
      <div>
        <h1 class="text-lg font-semibold text-slate-900">Sources</h1>
        <p class="text-sm text-slate-500">Fichiers détectés dans le répertoire de travail du serveur.</p>
      </div>
      <button
        class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
        :disabled="loadingMore"
        @click="refresh"
      >
        Rafraîchir
      </button>
    </div>

    <div
      class="mb-6 flex shrink-0 flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors"
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

    <DataTable :table="table" :loading="loadingMore" empty-message="Aucune source détectée.">
      <template #footer>
        <div ref="sentinel" class="h-px"></div>
        <p v-if="loadingMore" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
      </template>
    </DataTable>
  </div>
</template>
