import type { ColumnSchema, RowValue } from "../client/types";

export function columnType(columns: ColumnSchema[], name: string): string {
  return columns.find((c) => c.name === name)?.type ?? "";
}

// Text-ish columns are the ones whose values are worth suggesting: on a
// number, a date or a boolean the distinct list is either meaningless or
// unbounded, and the user types the value directly.
export function isTextType(type: string): boolean {
  return /CHAR|TEXT|STRING|UUID|ENUM/.test(type.toUpperCase());
}

export function coerceByType(type: string, raw: string): RowValue {
  const t = type.toUpperCase();
  if (t.includes("BOOL")) return raw === "true" || raw === "1";
  if (/INT|DOUBLE|FLOAT|DECIMAL|REAL|NUMERIC|HUGEINT/.test(t)) {
    if (raw === "") return null;
    const n = Number(raw);
    return Number.isNaN(n) ? raw : n;
  }
  return raw === "" ? null : raw;
}
