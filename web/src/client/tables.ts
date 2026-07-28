import { apiUrl, del, get, patch, post } from "./http";
import type { Metadata, Row, RowsPage, SchemaResponse } from "./types";

export function listTables(): Promise<Metadata[]> {
  return get("/tables");
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

export interface ColumnDTO {
  name: string;
  type: string;
}

export function listRows(tableId: string, offset: number, limit: number): Promise<RowsPage> {
  return get(`/tables/${tableId}/rows?offset=${offset}&limit=${limit}`);
}

export function getRow(tableId: string, pkey: string): Promise<{ row: Row }> {
  return get(`/tables/${tableId}/rows/${pkey}`);
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

export function exportUrl(tableId: string, format: "csv" | "geojson"): string {
  return apiUrl(`/tables/${tableId}/export?format=${format}`);
}
