import { reactive, ref, type Ref } from "vue";
import { addRows, deleteRow, getRowGeometry, listRows, updateRow } from "../client/tables";
import type { ColumnSchema, Row } from "../client/types";
import { errorMessage } from "../client/http";
import { coerceByType, columnType } from "../utils/columnTypes";
import { cellValue } from "../utils/rows";
import { usePaginatedList } from "./usePaginatedList";
import { useToast } from "./useToast";

// A table's rows: the current (unfiltered) page, server-side sorting,
// inline editing, insertion and deletion. Built on usePaginatedList for the
// paging/sorting/scroll-sentinel triplet rather than reimplementing it a
// fourth time (TODO-F4). onMutated is called after an add/delete, since
// that changes the table's rowCount — the caller owns table metadata
// (useTableMetadata) and reloads it.
export function useTableRows(
  tableId: Ref<string>,
  rowColumns: Ref<string[]>,
  columns: Ref<ColumnSchema[]>,
  geometryColumn: Ref<string | undefined>,
  onMutated: () => void,
) {
  const toast = useToast();
  const pageSize = 20;

  const list = usePaginatedList<Row>({
    limit: pageSize,
    key: tableId,
    fetchPage: (offset, limit, sort) =>
      listRows(tableId.value, offset, limit, sort).then((page) => ({ items: page.rows, total: page.total })),
    describeError: (e) => errorMessage(e, "Impossible de charger les lignes"),
  });

  // The physical column is named "__pkey", but Row objects key it as
  // "pkey" (router.rowToJSON pulls it from storage.Row's dedicated Pkey
  // field rather than its Values map) — map the two back together for
  // display rather than showing a blank cell.
  // --- Edit mode: toggling reveals a per-row "Modifier" action instead of
  // switching straight to a spreadsheet-like grid, matching the inline
  // edit/cancel pattern used everywhere else in this app (rename, EPSG). ---
  const editMode = ref(false);
  const editingPkey = ref<string | null>(null);
  const editValues = reactive<Record<string, string>>({});
  const savingRow = ref(false);
  const deletingPkey = ref<string | null>(null);

  function startEditRow(row: Row) {
    editingPkey.value = row.pkey;
    for (const col of rowColumns.value) {
      const v = cellValue(row, col);
      editValues[col] = v === null || v === undefined ? "" : String(v);
    }
  }

  function cancelEditRow() {
    editingPkey.value = null;
  }

  async function saveEditRow() {
    if (!editingPkey.value) return;
    savingRow.value = true;
    try {
      const updates: Record<string, unknown> = {};
      for (const col of rowColumns.value) {
        updates[col] = coerceByType(columnType(columns.value, col), editValues[col] ?? "");
      }
      await updateRow(tableId.value, editingPkey.value, updates);
      editingPkey.value = null;
      await list.reload();
      toast.success("Ligne mise à jour.");
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de mettre à jour la ligne"));
    } finally {
      savingRow.value = false;
    }
  }

  async function deleteOneRow(pkey: string) {
    deletingPkey.value = pkey;
    try {
      await deleteRow(tableId.value, pkey);
      await list.reload();
      onMutated();
      toast.success("Ligne supprimée.");
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de supprimer la ligne"));
    } finally {
      deletingPkey.value = null;
    }
  }

  // --- Duplicate row ---
  const duplicatingPkey = ref<string | null>(null);

  // A copy of the row under a new primary key: its attributes are taken from
  // the row already on screen, but its geometry has to be read for it. The
  // geometry column is deliberately absent from every row listing (LIM-F3),
  // and a geo row duplicated without it would land empty — a row that exists
  // in the grid and nowhere on the map.
  async function duplicateRow(row: Row) {
    duplicatingPkey.value = row.pkey;
    try {
      const values: Record<string, unknown> = {};
      for (const col of rowColumns.value) values[col] = cellValue(row, col);
      const column = geometryColumn.value;
      if (column) values[column] = (await getRowGeometry(tableId.value, row.pkey)).geometry;
      await addRows(tableId.value, [values]);
      await list.reload();
      onMutated();
      toast.success("Ligne dupliquée.");
    } catch (e) {
      toast.error(errorMessage(e, "Impossible de dupliquer la ligne"));
    } finally {
      duplicatingPkey.value = null;
    }
  }

  // --- Add row ---
  const addingRow = ref(false);
  const addRowError = ref<string | null>(null);

  // The typed values a row write expects, read off the editable columns —
  // the geometry column is not one of them: it is drawn on the map, not
  // typed (useGeometryEditing).
  function coerceValues(values: Record<string, string>): Record<string, unknown> {
    const coerced: Record<string, unknown> = {};
    for (const col of rowColumns.value) {
      coerced[col] = coerceByType(columnType(columns.value, col), values[col] ?? "");
    }
    return coerced;
  }

  async function addRow(values: Record<string, string>) {
    addingRow.value = true;
    addRowError.value = null;
    try {
      await addRows(tableId.value, [coerceValues(values)]);
      await list.reload();
      onMutated();
      toast.success("Ligne ajoutée.");
      return true;
    } catch (e) {
      addRowError.value = errorMessage(e, "Impossible d'ajouter la ligne");
      return false;
    } finally {
      addingRow.value = false;
    }
  }

  return {
    rows: list.items,
    total: list.total,
    pageSize,
    loading: list.loading,
    loadedOnce: list.loadedOnce,
    error: list.error,
    retry: list.retry,
    sorting: list.sorting,
    sentinel: list.sentinel,
    setSorting: list.setSorting,
    editMode,
    editingPkey,
    editValues,
    savingRow,
    deletingPkey,
    startEditRow,
    cancelEditRow,
    saveEditRow,
    deleteRow: deleteOneRow,
    duplicatingPkey,
    duplicateRow,
    addingRow,
    addRowError,
    addRow,
    coerceValues,
  };
}
