import type { SortingState } from "@tanstack/vue-table";

// columnMap translates a column id to the API's sort field name, for views
// whose display column ids (e.g. a formatted "size" column) don't match the
// backend's field name (e.g. "sizeBytes") one-to-one.
export function sortingToQuery(sorting: SortingState, columnMap?: Record<string, string>): string {
  return sorting.map((s) => `${s.desc ? "-" : ""}${columnMap?.[s.id] ?? s.id}`).join(",");
}
