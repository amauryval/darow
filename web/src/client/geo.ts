import { apiUrl, get, post } from "./http";

export function tileUrl(tableId: string): string {
  return apiUrl(`/tables/${tableId}/tiles/{z}/{x}/{y}`);
}

export function filterByBbox(tableId: string, bbox: [number, number, number, number]): Promise<{ pkeys: string[] }> {
  return post(`/tables/${tableId}/bbox`, { bbox });
}

// Distinct geometry types found in the table's geometry column — computed
// on demand server-side, never cached (a column can mix types).
export function getGeometryTypes(tableId: string): Promise<{ types: string[] }> {
  return get(`/tables/${tableId}/geometry-types`);
}
