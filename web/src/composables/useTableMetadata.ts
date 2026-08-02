import { computed, ref, watch, type Ref } from "vue";
import { addColumn, deleteTable, deriveTable, getSchema, getTable, renameTable } from "../client/tables";
import type { ColumnSchema, Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import { useToast } from "./useToast";

// A table's metadata and schema, reloaded whenever tableId changes (vue-router
// reuses TableDetailView across :tableId navigations, so this must react to
// the ref rather than run once in onMounted). Mutating actions (rename,
// delete, derive, addColumn) do the network call and rethrow on failure —
// callers decide whether the error becomes a toast or an inline form
// message, since that varies by call site (règle 8.4).
export function useTableMetadata(tableId: Ref<string>) {
  const toast = useToast();
  const table = ref<Metadata | null>(null);
  const columns = ref<ColumnSchema[]>([]);

  // Columns actually writable by AddRows/UpdateRow: everything but the
  // server-generated pkey. The geometry column (if any) stays in this list —
  // InsertRow/UpdateRow accept WKT for it — but it's excluded from
  // GetRows/GetRowsWithFilter, so it never has a value to display.
  const editableColumns = computed(() => columns.value.map((c) => c.name).filter((name) => name !== "__pkey"));
  const rowColumns = computed(() => editableColumns.value.filter((name) => name !== table.value?.geometryColumn));

  async function loadTable() {
    table.value = await getTable(tableId.value);
  }

  async function loadSchema() {
    const res = await getSchema(tableId.value);
    columns.value = res.columns;
  }

  async function loadAll() {
    try {
      await Promise.all([loadTable(), loadSchema()]);
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de charger la table"));
    }
  }

  async function rename(name: string) {
    table.value = await renameTable(tableId.value, name);
  }

  async function remove() {
    await deleteTable(tableId.value);
  }

  async function derive(query: string, newTableName: string) {
    return deriveTable(tableId.value, query, newTableName);
  }

  async function addNewColumn(name: string, type: string) {
    await addColumn(tableId.value, name, type);
    await loadSchema();
  }

  watch(tableId, loadAll, { immediate: true });

  return { table, columns, editableColumns, rowColumns, loadTable, rename, remove, derive, addNewColumn };
}
