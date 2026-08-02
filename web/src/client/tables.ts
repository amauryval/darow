import { del, get, getBlob, patch, post } from "./http";
import type { ColumnDTO, FilterCondition, Metadata, Row, RowsPage, RowValue, SchemaResponse, TablesPage } from "./types";

export function listTables(offset: number, limit: number, sort?: string): Promise<TablesPage> {
  const sortParam = sort ? `&sort=${encodeURIComponent(sort)}` : "";
  return get(`/tables?offset=${offset}&limit=${limit}${sortParam}`);
}

export function getTable(tableId: string): Promise<Metadata> {
  return get(`/tables/${tableId}`);
}

export function deleteTable(tableId: string): Promise<void> {
  return del(`/tables/${tableId}`);
}

export function renameTable(tableId: string, tableName: string): Promise<Metadata> {
  return patch(`/tables/${tableId}`, { tableName });
}

export function getSchema(tableId: string): Promise<SchemaResponse> {
  return get(`/tables/${tableId}/schema`);
}

export function addColumn(tableId: string, name: string, type: string): Promise<ColumnDTO> {
  return post(`/tables/${tableId}/columns`, { name, type });
}

export function listRows(tableId: string, offset: number, limit: number, sort?: string): Promise<RowsPage> {
  const sortParam = sort ? `&sort=${encodeURIComponent(sort)}` : "";
  return get(`/tables/${tableId}/rows?offset=${offset}&limit=${limit}${sortParam}`);
}

export function getRow(tableId: string, pkey: string): Promise<{ row: Row }> {
  return get(`/tables/${tableId}/rows/${pkey}`);
}

// Fetches several rows by pkey in a single request — used to hydrate only
// the current page of an arbitrary pkey list (e.g. the map's selected
// features) instead of one GetRow call per row.
export function getRowsByPkey(tableId: string, pkeys: string[]): Promise<{ rows: Row[] }> {
  return post(`/tables/${tableId}/rows/batch`, { pkeys });
}

// Inserts one or more rows in a single request; returns the new rows'
// primary keys in insertion order.
export function addRows(tableId: string, rows: Array<Record<string, unknown>>): Promise<{ pkeys: string[] }> {
  return post(`/tables/${tableId}/rows`, { rows });
}

export function updateRow(tableId: string, pkey: string, updates: Record<string, unknown>): Promise<{ row: Row }> {
  return patch(`/tables/${tableId}/rows/${pkey}`, updates);
}

export function deleteRow(tableId: string, pkey: string): Promise<void> {
  return del(`/tables/${tableId}/rows/${pkey}`);
}

export function deriveTable(tableId: string, query: string, newTableName: string): Promise<{ tableId: string }> {
  return post(`/tables/${tableId}/derive`, { query, newTableName });
}

// Streams a table export and triggers a browser download through this
// client (getBlob), instead of a plain <a href> that bypasses it entirely:
// that left a failure to show up as a bare JSON error tab, or a truncated
// file with no indication anything went wrong. Rejects like any other
// client call — the caller turns that into the same errorMessage/toast
// every other action uses.
export async function exportTable(tableId: string, format: "csv" | "geojson", tableName: string): Promise<void> {
  const blob = await getBlob(`/tables/${tableId}/export?format=${format}`);
  const url = URL.createObjectURL(blob);
  try {
    const link = document.createElement("a");
    link.href = url;
    link.download = `${tableName}.${format}`;
    link.click();
  } finally {
    URL.revokeObjectURL(url);
  }
}

// Filters rows by column conditions, ANDed together — a server-validated
// alternative to building a derive() SELECT by hand. An omitted or
// non-positive limit falls back to the server's default row limit.
export function filterRows(tableId: string, conditions: FilterCondition[], limit?: number): Promise<{ rows: Row[] }> {
  return post(`/tables/${tableId}/filter`, { conditions, limit });
}

// Lists a column's distinct values, paginated server-side. `search` keeps
// only the values containing it (case-insensitive, wildcards literal), which
// is what makes this usable as a value-suggestion source on a high-cardinality
// column: the server returns the first `limit` matches, never everything.
export function getDistinctValues(
  tableId: string,
  col: string,
  opts: { search?: string; limit?: number } = {},
): Promise<{ values: RowValue[] }> {
  const params = new URLSearchParams();
  if (opts.search) params.set("search", opts.search);
  if (opts.limit !== undefined) params.set("limit", String(opts.limit));
  const query = params.size > 0 ? `?${params}` : "";
  return get(`/tables/${tableId}/columns/${encodeURIComponent(col)}/distinct${query}`);
}
