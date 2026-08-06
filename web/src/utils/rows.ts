import type { Row, RowValue } from '../client/types'

// The primary key is carried by Row.pkey, not by a "__pkey" entry in the row
// itself, so a column list that includes it has to be read through here.
export function cellValue(row: Row, col: string): RowValue {
    return col === '__pkey' ? row.pkey : row[col]
}

// Columns the server manages internally and never surfaces as real data:
// "__pkey" (the generated primary key) and "OGC_FID" (GDAL/OGR's injected
// feature-id column, present on almost every imported file regardless of
// geometry — see storage.IsHiddenColumn server-side). Both stay physically
// stored; only display excludes them.
const HIDDEN_COLUMNS = new Set(['__pkey', 'OGC_FID'])

export function isHiddenColumn(name: string): boolean {
    return HIDDEN_COLUMNS.has(name)
}
