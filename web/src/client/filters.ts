import { del, get, patch, post } from "./http";
import type { FilterCondition, SavedFilter, SavedFiltersPage } from "./types";

export function listSavedFilters(tableId: string): Promise<SavedFiltersPage> {
  return get(`/tables/${tableId}/filters`);
}

export function createSavedFilter(
  tableId: string,
  name: string,
  conditions: FilterCondition[],
): Promise<SavedFilter> {
  return post(`/tables/${tableId}/filters`, { name, conditions });
}

// An omitted field is left untouched server-side: that is what distinguishes
// renaming a filter from replacing its conditions, so the caller passes only
// what actually changed.
export function updateSavedFilter(
  tableId: string,
  filterId: string,
  patchBody: { name?: string; conditions?: FilterCondition[] },
): Promise<SavedFilter> {
  return patch(`/tables/${tableId}/filters/${filterId}`, patchBody);
}

export function deleteSavedFilter(tableId: string, filterId: string): Promise<void> {
  return del(`/tables/${tableId}/filters/${filterId}`);
}
