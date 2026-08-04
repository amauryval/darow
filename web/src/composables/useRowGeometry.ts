import { ref, type Ref } from "vue";
import { getRowGeometry } from "../client/tables";
import { errorMessage } from "../client/http";
import { useToast } from "./useToast";

// One row's geometry, read on demand: the geometry column is deliberately
// absent from every row listing, so this is the one path that shows it —
// GeoJSON to read or reuse it, WKT to copy it into another tool.
export function useRowGeometry(tableId: Ref<string>) {
  const toast = useToast();
  const pkey = ref<string | null>(null);
  const geometry = ref<GeoJSON.Geometry | null>(null);
  const wkt = ref("");
  const loading = ref(false);
  const error = ref("");

  async function load(rowPkey: string) {
    pkey.value = rowPkey;
    loading.value = true;
    error.value = "";
    try {
      const result = await getRowGeometry(tableId.value, rowPkey);
      // A later open() while this one was in flight already owns the modal.
      if (pkey.value !== rowPkey) return;
      geometry.value = result.geometry;
      wkt.value = result.wkt;
    } catch (e) {
      if (pkey.value !== rowPkey) return;
      error.value = errorMessage(e, "Impossible de charger la géométrie de cette ligne");
    } finally {
      if (pkey.value === rowPkey) loading.value = false;
    }
  }

  function open(rowPkey: string) {
    geometry.value = null;
    wkt.value = "";
    load(rowPkey);
  }

  function retry() {
    if (pkey.value) load(pkey.value);
  }

  function close() {
    pkey.value = null;
    geometry.value = null;
    wkt.value = "";
    error.value = "";
  }

  async function copy(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`${label} copié dans le presse-papiers.`);
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de copier dans le presse-papiers"));
    }
  }

  return { pkey, geometry, wkt, loading, error, open, retry, close, copy };
}
