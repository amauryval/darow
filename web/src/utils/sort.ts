import type { SortingState } from "@tanstack/vue-table";

// Projection of TanStack's sorting state onto the API's `sort` parameter
// ("-field" for desc, "field" for asc, comma-separated) and back, so the
// same compact format can also live in the URL query string (TODO-F23).
export function stringifySorting(sorting: SortingState, columnMap?: Record<string, string>): string {
  return sorting.map((s) => `${s.desc ? "-" : ""}${columnMap?.[s.id] ?? s.id}`).join(",");
}

export function parseSorting(param: string): SortingState {
  return param
    .split(",")
    .filter(Boolean)
    .map((s) => (s.startsWith("-") ? { id: s.slice(1), desc: true } : { id: s, desc: false }));
}
