import { ref } from "vue";
import { deleteSource, getSourceMetadata, listSources, loadSource, refreshSources, uploadSource } from "../client/sources";
import type { Source } from "../client/types";
import { errorMessage } from "../client/http";
import { usePaginatedList } from "./usePaginatedList";
import { useToast } from "./useToast";

// Display column ids that don't match the API's Source field names 1:1
// ("size" formats sizeBytes+layerCount, "status" formats loaded).
const sourceSortColumnMap: Record<string, string> = { size: "sizeBytes", status: "loaded" };

export function layerKey(source: Source, layerName: string): string {
  return `${source.name}::${layerName}`;
}

// workDir source list, upload, per-layer loading (with EPSG override) and
// deletion. Built on usePaginatedList for the paging/sorting/scroll-sentinel
// triplet (TODO-F4).
export function useSources() {
  const toast = useToast();

  const limit = 50;
  const list = usePaginatedList<Source>({
    limit,
    sortColumnMap: sourceSortColumnMap,
    fetchPage: (offset, limit, sort) => listSources(offset, limit, sort).then((page) => ({ items: page.sources, total: page.total })),
    onError: (e) => toast.error(errorMessage(e, "Impossible de charger les sources")),
  });

  // Distinct from reload(): re-scans workDir for new/removed files instead
  // of just re-fetching the already-known list.
  async function refresh() {
    try {
      const page = await refreshSources(0, limit, list.sortQuery());
      list.replace({ items: page.sources, total: page.total });
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de rafraîchir les sources"));
    }
  }

  // --- Upload ---
  const uploading = ref(false);

  // Resolves to { tableId } when the file was loaded straight away, or null
  // for a multi-layer GeoPackage (no layer auto-selected — the caller finds
  // it back in the reloaded list to let the user pick a layer). Rethrows on
  // failure so the caller can word the error with the file name.
  async function upload(file: File): Promise<{ tableId: string } | null> {
    uploading.value = true;
    try {
      const result = await uploadSource(file);
      if ("tableId" in result && result.tableId !== undefined) return { tableId: result.tableId };
      await list.reload();
      return null;
    } finally {
      uploading.value = false;
    }
  }

  // --- Load (EPSG detection + confirmation) ---
  const epsgEditingKey = ref<string | null>(null);
  const epsgInput = ref("");
  const epsgDetecting = ref(false);
  const loadingKey = ref<string | null>(null);

  async function startLoad(source: Source, layerName?: string) {
    const key = layerName ? layerKey(source, layerName) : source.name;
    epsgEditingKey.value = key;
    epsgInput.value = "";
    epsgDetecting.value = true;
    try {
      const crs = await getSourceMetadata(source.name, layerName);
      if (epsgEditingKey.value === key) epsgInput.value = crs.srid ? String(crs.srid) : "";
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
    loadingKey.value = key;
    try {
      await loadSource(source.name, layerName, Number(epsgInput.value));
      epsgEditingKey.value = null;
      await list.reload();
      toast.success(`${key} chargé.`);
    } catch (e) {
      toast.error(errorMessage(e, `Impossible de charger ${key}`));
    } finally {
      loadingKey.value = null;
    }
  }

  // --- Delete ---
  const deletingName = ref<string | null>(null);

  async function remove(name: string) {
    deletingName.value = name;
    try {
      await deleteSource(name);
      await list.reload();
      toast.success(`${name} supprimé.`);
    } catch (e) {
      toast.error(errorMessage(e, `Impossible de supprimer ${name}`));
    } finally {
      deletingName.value = null;
    }
  }

  return {
    sources: list.items,
    total: list.total,
    loading: list.loading,
    sorting: list.sorting,
    sentinel: list.sentinel,
    setSorting: list.setSorting,
    reload: list.reload,
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
  };
}
