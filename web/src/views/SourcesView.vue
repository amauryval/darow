<script setup lang="ts">
import { computed, h, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { createColumnHelper, getCoreRowModel, getExpandedRowModel, useVueTable } from "@tanstack/vue-table";
import type { Source } from "../client/types";
import { errorMessage } from "../client/http";
import { layerKey, useSources } from "../composables/useSources";
import { useToast } from "../composables/useToast";
import { useDocumentTitle } from "../composables/useDocumentTitle";
import { parseSorting, stringifySorting } from "../utils/sort";
import { queryString } from "../utils/routeQuery";
import { UploadCloud } from "@lucide/vue";
import Badge from "../components/Badge.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import DataTable from "../components/DataTable.vue";
import ErrorState from "../components/ui/ErrorState.vue";
import Skeleton from "../components/ui/Skeleton.vue";

const router = useRouter();
const route = useRoute();
const toast = useToast();

useDocumentTitle(computed(() => "Sources"));

// sources.sentinel stays nested rather than destructured: the template ref
// bound via `:ref` needs the actual Ref object, but a top-level destructured
// ref auto-unwraps when read inside the template.
const sourcesState = useSources(queryString(route.query.sort) ? parseSorting(queryString(route.query.sort)) : undefined);
const {
  sources,
  loading,
  loadedOnce,
  error,
  retry,
  sorting,
  setSorting,
  refresh,
  uploading,
  upload,
  epsgEditingKey,
  epsgInput,
  epsgDetecting,
  loadingKey,
  startLoad,
  cancelLoad,
  confirmLoad,
  deletingName,
  remove,
} = sourcesState;

// The URL carries the screen state (§9): a refresh or a shared link keeps
// the chosen sort.
watch(sorting, (sort) => {
  router.replace({ query: { ...route.query, sort: stringifySorting(sort) || undefined } });
});

const dragging = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

// Stated before the attempt, not only in an error response (§8.5). Static
// text matching the server's default config.yaml (sources.maxSizeBytes /
// sources.allowedExtensions) — there is no endpoint exposing the running
// deployment's actual values, so a custom config drifts from this hint.
const ACCEPTED_EXTENSIONS = [".geojson", ".shp", ".csv", ".gpkg", ".parquet"];
const MAX_UPLOAD_LABEL = "100 Mo";

function isValidEpsg(value: string): boolean {
  return /^[1-9]\d*$/.test(value.trim());
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

function triggerUpload() {
  fileInput.value?.click();
}

async function handleUpload(file: File) {
  try {
    const result = await upload(file);
    if (result) {
      toast.success(`${file.name} chargé.`);
      router.push(`/tables/${result.tableId}`);
    } else {
      toast.success(`${file.name} ajouté — choisissez une couche à charger.`);
    }
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de charger ${file.name}`));
  }
}

async function onFileSelected(event: Event) {
  const file = event.target instanceof HTMLInputElement ? event.target.files?.[0] : undefined;
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
      row.original.layerCount !== undefined ? null : h(Badge, { tone: row.original.loaded ? "success" : "neutral" }, () => (row.original.loaded ? "Chargée" : "Non chargée")),
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
            onConfirm: () => remove(r.source.name),
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
            onInput: (e: Event) => {
              if (e.target instanceof HTMLInputElement) {
                epsgInput.value = e.target.value;
              }
            },
            class: "w-24 rounded-md border border-slate-300 px-2 py-1 text-xs disabled:bg-slate-50",
          }),
          h(
            "button",
            {
              class: "rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50",
              disabled: epsgDetecting.value || !isValidEpsg(epsgInput.value) || loadingKey.value === key,
              onClick: () => confirmLoad(r.source, r.layerName),
            },
            loadingKey.value === key ? "Chargement..." : "Confirmer",
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
            onClick: () => startLoad(r.source, r.layerName),
          },
          "Charger",
        );
      }
      return h("div", { class: "flex items-center justify-end gap-2" }, [
        h(
          "button",
          {
            class: "rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50",
            onClick: () => startLoad(r.source),
          },
          "Charger",
        ),
        h(ConfirmButton, { label: "Supprimer", busy: deletingName.value === r.source.name, onConfirm: () => remove(r.source.name) }),
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
  onSortingChange: setSorting,
});
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col px-6 py-4">
    <div class="mb-4 flex shrink-0 items-center justify-between">
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
      class="mb-4 flex shrink-0 flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-8 text-center transition-colors"
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
      <p class="text-xs text-slate-400">{{ ACCEPTED_EXTENSIONS.join(", ") }} — {{ MAX_UPLOAD_LABEL }} maximum</p>
    </div>

    <Skeleton v-if="loading && !loadedOnce" />
    <ErrorState v-else-if="error" :message="error" @retry="retry" />
    <DataTable v-else :table="table" :loading="loading" empty-message="Aucune source détectée.">
      <template #footer>
        <div :ref="sourcesState.sentinel" class="h-px"></div>
        <p v-if="loading" class="px-4 py-3 text-center text-xs text-slate-400">Chargement…</p>
      </template>
    </DataTable>
  </div>
</template>
