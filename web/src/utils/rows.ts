import type { Row, RowValue } from "../client/types";

// The primary key is carried by Row.pkey, not by a "__pkey" entry in the row
// itself, so a column list that includes it has to be read through here.
export function cellValue(row: Row, col: string): RowValue {
  return col === "__pkey" ? row.pkey : row[col];
}
