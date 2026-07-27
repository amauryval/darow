import { post } from "./http";

export function tileUrl(tableId: string): string {
  return `/tables/${tableId}/tiles/{z}/{x}/{y}`;
}

export function filterByBbox(tableId: string, bbox: [number, number, number, number]): Promise<{ pkeys: string[] }> {
  return post(`/tables/${tableId}/bbox`, { bbox });
}
