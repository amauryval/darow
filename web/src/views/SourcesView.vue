<script setup lang="ts">
import { computed, h, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createColumnHelper, getCoreRowModel, getExpandedRowModel, useVueTable } from "@tanstack/vue-table";
import { errorMessage } from "../client/http";
import { layerKey, useSources } from "../composables/useSources";
import { useToast } from "../composables/useToast";
import { useDocumentTitle } from "../composables/useDocumentTitle";
import { parseSorting, stringifySorting } from "../utils/sort";
import { queryString } from "../utils/routeQuery";
import { formatSize, toSourceRows } from "../utils/sourceRows";
import type { SourceRow } from "../utils/sourceRows";
import { RefreshCw, UploadCloud } from "@lucide/vue";
import Badge from "../components/Badge.vue";
import DataTable from "../components/DataTable.vue";
import ErrorState from "../components/ui/ErrorState.vue";
import Skeleton from "../components/ui/Skeleton.vue";
import BaseButton from "../components/ui/BaseButton.vue";
import Breadcrumbs from "../components/ui/Breadcrumbs.vue";
import Typography from "../components/ui/Typography.vue";
import SourceActionsCell from "../components/sources/SourceActionsCell.vue";
import SourceNameCell from "../components/sources/SourceNameCell.vue";
import SourceUploadModal from "../components/sources/SourceUploadModal.vue";

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
  total,
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

const uploadOpen = ref(false);

const rows = computed<SourceRow[]>(() => toSourceRows(sources.value));

// Counted on the loaded page only — the list is paginated, so this describes
// what is on screen, not the whole workDir.
const loadedCount = computed(() => sources.value.filter((source) => source.loaded).length);

async function handleUpload(file: File) {
  try {
    const result = await upload(file);
    uploadOpen.value = false;
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

function rowKey(row: SourceRow): string {
  return row.layerName ? layerKey(row.source, row.layerName) : row.source.name;
}

const columnHelper = createColumnHelper<SourceRow>();
const columnDefs = [
  columnHelper.accessor("name", {
    header: "Nom",
    cell: ({ row, getValue }) =>
      h(SourceNameCell, {
        name: getValue(),
        isLayer: row.depth > 0,
        layerCount: row.original.layerCount,
        ext: row.original.source.ext,
      }),
  }),
  columnHelper.display({
    id: "size",
    header: "Taille",
    enableSorting: true,
    cell: ({ row }) =>
      row.original.layerCount !== undefined
        ? h("span", { class: "text-xs text-base-content/50" }, `${row.original.layerCount} couches · ${formatSize(row.original.source.sizeBytes)}`)
        : row.original.sizeBytes !== undefined
          ? h("span", { class: "text-base-content/60" }, formatSize(row.original.sizeBytes))
          : null,
  }),
  columnHelper.display({
    id: "status",
    header: "Statut",
    enableSorting: true,
    cell: ({ row }) =>
      row.original.layerCount !== undefined
        ? null
        : h(Badge, { tone: row.original.loaded ? "success" : "neutral" }, () => (row.original.loaded ? "Chargée" : "Non chargée")),
  }),
  columnHelper.display({
    id: "actions",
    header: "",
    meta: { align: "right" },
    cell: ({ row }) =>
      h(SourceActionsCell, {
        row: row.original,
        editingEpsg: epsgEditingKey.value === rowKey(row.original),
        epsg: epsgInput.value,
        detecting: epsgDetecting.value,
        loadingRow: loadingKey.value === rowKey(row.original),
        deleting: deletingName.value === row.original.source.name,
        "onUpdate:epsg": (value: string) => (epsgInput.value = value),
        onStartLoad: () => startLoad(row.original.source, row.original.layerName),
        onCancelLoad: cancelLoad,
        onConfirmLoad: () => confirmLoad(row.original.source, row.original.layerName),
        onRemove: () => remove(row.original.source.name),
      }),
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
  <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
    <Breadcrumbs :items="[{ label: 'Sources' }]" />

    <div class="flex shrink-0 flex-wrap items-center justify-between gap-3">
      <div class="flex flex-col gap-1">
        <Typography variant="body-sm">Fichiers détectés dans le répertoire de travail du serveur.</Typography>
        <div v-if="loadedOnce" class="flex items-center gap-2">
          <Badge tone="neutral">{{ total }} fichier{{ total > 1 ? "s" : "" }}</Badge>
          <Badge v-if="loadedCount > 0" tone="success">{{ loadedCount }} chargé{{ loadedCount > 1 ? "s" : "" }}</Badge>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <BaseButton variant="secondary" size="sm" :icon="RefreshCw" :disabled="loading" @click="refresh">Rafraîchir</BaseButton>
        <BaseButton variant="primary" size="sm" :icon="UploadCloud" :busy="uploading" :disabled="uploading" @click="uploadOpen = true">
          Importer un fichier
        </BaseButton>
      </div>
    </div>

    <Skeleton v-if="loading && !loadedOnce" />
    <ErrorState v-else-if="error" :message="error" @retry="retry" />
    <DataTable v-else :table="table" :loading="loading" empty-message="Aucune source détectée — importez un fichier pour commencer.">
      <template #footer>
        <div :ref="sourcesState.sentinel" class="h-px"></div>
        <p v-if="loading" class="px-4 py-3 text-center text-xs text-base-content/40">Chargement…</p>
      </template>
    </DataTable>

    <SourceUploadModal v-if="uploadOpen" :uploading="uploading" @close="uploadOpen = false" @upload="handleUpload" />
  </div>
</template>
