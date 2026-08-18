import { del, get, getBlob, patch, post } from './http'
import type {
    AggregateSpec,
    ColumnDTO,
    Deleted,
    DeriveSpec,
    ExportFormat,
    FilterCondition,
    Metadata,
    Row,
    RowGeometry,
    RowsPage,
    RowValue,
    SchemaResponse,
    TableProvenance,
    TablesPage,
    TransformPreview,
    TransformResult,
    TransformSpec
} from './types'

export function listTables(offset: number, limit: number, sort?: string): Promise<TablesPage> {
    const sortParam = sort ? `&sort=${encodeURIComponent(sort)}` : ''
    return get(`/tables?offset=${offset}&limit=${limit}${sortParam}`)
}

// The whole catalogue's provenance in one call, so the sources screen can tell
// which files already have a table without paging through /tables.
export function listTableProvenance(): Promise<{ tables: TableProvenance[] }> {
    return get('/tables/provenance')
}

export function getTable(tableId: string): Promise<Metadata> {
    return get(`/tables/${tableId}`)
}

export function deleteTable(tableId: string): Promise<Deleted> {
    return del(`/tables/${tableId}`)
}

export function renameTable(tableId: string, tableName: string): Promise<Metadata> {
    return patch(`/tables/${tableId}`, { tableName })
}

export function getSchema(tableId: string): Promise<SchemaResponse> {
    return get(`/tables/${tableId}/schema`)
}

export function addColumn(tableId: string, name: string, type: string): Promise<ColumnDTO> {
    return post(`/tables/${tableId}/columns`, { name, type })
}

// Renaming and dropping act on the structure, not on what is displayed: the
// caller reloads the schema afterwards, as it does after addColumn.
export function renameColumn(
    tableId: string,
    col: string,
    name: string
): Promise<{ name: string }> {
    return patch(`/tables/${tableId}/columns/${encodeURIComponent(col)}`, { name })
}

export function dropColumn(tableId: string, col: string): Promise<{ name: string }> {
    return del(`/tables/${tableId}/columns/${encodeURIComponent(col)}`)
}

export function listRows(
    tableId: string,
    offset: number,
    limit: number,
    sort?: string
): Promise<RowsPage> {
    const sortParam = sort ? `&sort=${encodeURIComponent(sort)}` : ''
    return get(`/tables/${tableId}/rows?offset=${offset}&limit=${limit}${sortParam}`)
}

// Fetches several rows by pkey in a single request — used to hydrate only
// the current page of an arbitrary pkey list (e.g. the map's selected
// features) instead of one GetRow call per row.
export function getRowsByPkey(tableId: string, pkeys: string[]): Promise<{ rows: Row[] }> {
    return post(`/tables/${tableId}/rows/batch`, { pkeys })
}

// A geometry is written in the CRS the map speaks (lon/lat), which is what
// every read emits too — the server reprojects to what it stores. Naming
// another CRS is the caller's business, never a conversion done here.
function crsParam(crs?: string): string {
    return crs ? `?crs=${encodeURIComponent(crs)}` : ''
}

// Inserts one or more rows in a single request, returning the new pkeys in
// insertion order. A single insert setting the geometry echoes back what was
// stored, to be drawn instead of the draft that was sent.
export function addRows(
    tableId: string,
    rows: Array<Record<string, unknown>>,
    crs?: string
): Promise<{ pkeys: string[]; geometry?: GeoJSON.Geometry }> {
    return post(`/tables/${tableId}/rows${crsParam(crs)}`, { rows })
}

export function updateRow(
    tableId: string,
    pkey: string,
    updates: Record<string, unknown>,
    crs?: string
): Promise<{ row: Row; geometry?: GeoJSON.Geometry }> {
    return patch(`/tables/${tableId}/rows/${pkey}${crsParam(crs)}`, updates)
}

// Several rows in one transaction: all land or none. This is what a
// topological edit writes through — a shared boundary moves in every row
// carrying the vertex, and half of that landing is a corrupted boundary.
export function updateRows(
    tableId: string,
    updates: Array<{ pkey: string; values: Record<string, unknown> }>,
    crs?: string
): Promise<{ rows: Array<{ pkey: string; geometry?: GeoJSON.Geometry }> }> {
    return patch(`/tables/${tableId}/rows${crsParam(crs)}`, { updates })
}

// A single row's geometry, in both encodings: GeoJSON to draw or edit it,
// WKT to read and copy it. Full resolution, lon/lat — the geometry is
// deliberately absent from every other row read.
// srid reprojects what is read, exactly as it does for an export, and is left
// out to get the table's own CRS. Nothing stored changes either way.
export function getRowGeometry(
    tableId: string,
    pkey: string,
    srid?: number | null
): Promise<RowGeometry> {
    const qs = srid ? `?srid=${srid}` : ''
    return get(`/tables/${tableId}/rows/${pkey}/geometry${qs}`)
}

export function deleteRow(tableId: string, pkey: string): Promise<Deleted> {
    return del(`/tables/${tableId}/rows/${pkey}`)
}

// Deletes a set of rows in a single write rather than one request per row.
export function deleteRows(
    tableId: string,
    pkeys: string[]
): Promise<{ pkeys: string[]; rowsRemoved: number }> {
    return del(`/tables/${tableId}/rows`, { pkeys })
}

// Free-SQL derivation: a top-level route, scoped to no table — the query may
// read any registered table. deriveTableFromTable below is the
// single-source mode.
export function deriveTable(query: string, newTableName: string): Promise<{ tableId: string }> {
    return post(`/derive`, { query, newTableName })
}

// Guided derivation: the source is tableId and nothing else, and the server
// assembles the SQL from the spec.
export function deriveTableFromTable(
    tableId: string,
    spec: DeriveSpec
): Promise<{ tableId: string }> {
    return post(`/tables/${tableId}/derive-from-table`, spec)
}

// Guided aggregation: a GROUP BY over tableId's columns, the aggregation
// counterpart of deriveTableFromTable.
export function aggregateTableFromTable(
    tableId: string,
    spec: AggregateSpec
): Promise<{ tableId: string }> {
    return post(`/tables/${tableId}/aggregate-from-table`, spec)
}

// Streams an export and triggers the download through this client (getBlob),
// not a plain <a href> that bypasses it: a failure would otherwise land as a
// bare JSON tab. Rejects like any other call, for the caller's usual toast.
export async function exportTable(
    tableId: string,
    format: ExportFormat,
    srid: number | null,
    tableName: string
): Promise<void> {
    const params = new URLSearchParams({ format })
    if (srid !== null) params.set('srid', String(srid))

    const { blob, filename } = await getBlob(`/tables/${tableId}/export?${params}`)
    const url = URL.createObjectURL(blob)
    try {
        const link = document.createElement('a')
        link.href = url
        link.download = filename ?? `${tableName}.${format}`
        link.click()
    } finally {
        URL.revokeObjectURL(url)
    }
}

// Filters rows by column conditions, ANDed together — a server-validated
// alternative to building a derive() SELECT by hand. An omitted or
// non-positive limit falls back to the server's default row limit.
export function filterRows(
    tableId: string,
    conditions: FilterCondition[],
    limit?: number
): Promise<{ rows: Row[] }> {
    return post(`/tables/${tableId}/filter`, { conditions, limit })
}

// A column's distinct values, paginated server-side. `search` keeps only the
// values containing it (case-insensitive, wildcards literal), which is what
// makes this a suggestion source on a high-cardinality column.
export function getDistinctValues(
    tableId: string,
    col: string,
    opts: { search?: string; limit?: number } = {}
): Promise<{ values: RowValue[] }> {
    const params = new URLSearchParams()
    if (opts.search) params.set('search', opts.search)
    if (opts.limit !== undefined) params.set('limit', String(opts.limit))
    const query = params.size > 0 ? `?${params}` : ''
    return get(`/tables/${tableId}/columns/${encodeURIComponent(col)}/distinct${query}`)
}

// Rewrites col's values in place, on the rows spec.conditions names —
// irreversible (§8.3), which is why previewColumnTransform below shares the
// same spec and exists to be called first.
export function transformColumn(
    tableId: string,
    col: string,
    spec: TransformSpec
): Promise<TransformResult> {
    return post(`/tables/${tableId}/columns/${encodeURIComponent(col)}/transform`, spec)
}

// How many rows spec would touch and a bounded before/after sample, writing
// nothing — what stands in for an undo a column transform does not have.
export function previewColumnTransform(
    tableId: string,
    col: string,
    spec: TransformSpec
): Promise<TransformPreview> {
    return post(`/tables/${tableId}/columns/${encodeURIComponent(col)}/transform/preview`, spec)
}
