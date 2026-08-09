export interface Diagnostics {
    databaseSizeBytes: number
    memoryUsageBytes: number
    temporaryFilesBytes: number
    extensions: string[]
}

export interface Metadata {
    tableId: string
    physicalName: string
    tableName: string
    isGeo: boolean
    createdAt: string
    updatedAt: string
    geometryColumn?: string
    rowCount: number
    bbox?: string
    srid: number
}

interface SourceLayer {
    name: string
    loaded: boolean
    tableId?: string
}

export interface Source {
    name: string
    ext: string
    sizeBytes: number
    modTime: string
    loaded: boolean
    tableId?: string
    layers?: SourceLayer[]
}

export interface SourceCRS {
    srid: number | null
    authName: string
    // Whether the source carries a geometry field at all. It decides whether a
    // CRS has to be asked for: a file of plain columns has nothing to project,
    // and the server takes it with no srid.
    hasGeometry: boolean
}

export interface TablesPage {
    tables: Metadata[]
    total: number
    limit: number
    offset: number
}

export interface SourcesPage {
    sources: Source[]
    total: number
    limit: number
    offset: number
}

export type RowValue = string | number | boolean | null

export interface Row {
    pkey: string
    [column: string]: RowValue
}

// A row's geometry, as served by the per-row geometry route: GeoJSON to
// draw it, WKT to read it. Both at full resolution, in lon/lat.
export interface RowGeometry {
    pkey: string
    geometry: GeoJSON.Geometry | null
    wkt: string
}

export interface RowsPage {
    rows: Row[]
    total: number
    limit: number
    offset: number
}

export interface ColumnSchema {
    name: string
    type: string
}

export interface SchemaResponse {
    columns: ColumnSchema[]
}

// Matches storage.FilterCondition — a node is either a leaf
// (column/operator/value) or a boolean group (bool + conditions), never
// both. Column and operator are validated server-side against the table's
// real schema and a fixed operator whitelist, so no enum is enforced
// client-side.
export interface FilterCondition {
    column?: string
    operator?: string
    value?: RowValue
    // Negates the node, leaf or group alike.
    not?: boolean
    bool?: 'AND' | 'OR'
    conditions?: FilterCondition[]
}

// One column a guided derivation computes rather than carries over: a free
// SQL scalar expression over the source table's columns, under a chosen
// name. The expression is validated server-side (DuckDB's own parser), so
// nothing checks it here beyond it being non-empty.
export interface ComputedColumn {
    alias: string
    expression: string
}

// Matches service.DeriveSpec — a derivation from one source table, described
// structurally instead of as SQL. Every field but newTableName is optional:
// an empty spec derives the whole source table. The SQL is assembled
// server-side, which is why no query string appears here.
export interface DeriveSpec {
    newTableName: string
    columns?: string[]
    computed?: ComputedColumn[]
    conditions?: FilterCondition[]
}

// A filter saved against a table, server-side. Its conditions are the same
// tree an ad-hoc filter posts, validated against the table's real schema at
// save time — so a stored filter is always applicable. Deleting the table
// deletes its filters.
export interface SavedFilter {
    filterId: string
    tableId: string
    name: string
    conditions: FilterCondition[]
    createdAt: string
    updatedAt: string
}

export interface SavedFiltersPage {
    filters: SavedFilter[]
    total: number
}

export interface ColumnDTO {
    name: string
    type: string
}

export interface ErrorResponse {
    error: string
    message: string
    code: number
    requestId: string
}

// Matches storage.Settings — every server parameter an operator legitimately
// retunes while the process is serving, without a restart. What Config keeps
// instead (TODO-17): the listen port, the DuckDB file path, sources.workDir,
// and the memory/availability guards that sit on this table's own
// unauthenticated PATCH route (the request body cap, the per-request
// timeout, every LRU cache's size) — none of that appears here, so it has no
// place on a screen of *editable* settings.
// The three max*Limit fields are one editable limit per family of pages,
// 0 meaning "the server's own built-in limits"; the six effective* fields are
// read-only, the pair the server actually applies (page size when a request
// asks for none, ceiling when it asks for more), so no caller has to hold a
// copy of the server's constants.
export interface Settings {
    maxRowLimit: number
    maxListLimit: number
    maxFeaturesLimit: number
    maxSimplifyTolerance: number
    maxPrecision: number
    tileCacheMaxAgeSec: number
    cacheTTLSec: number
    coordinateDecimals: number
    effectiveDefaultRowLimit: number
    effectiveMaxRowLimit: number
    effectiveDefaultListLimit: number
    effectiveMaxListLimit: number
    effectiveDefaultFeaturesLimit: number
    effectiveMaxFeaturesLimit: number
    updatedAt: string
}

// The read-only half of Settings: derived server-side, never sent back.
export type SettingsReadOnlyKey =
    | 'updatedAt'
    | 'effectiveDefaultRowLimit'
    | 'effectiveMaxRowLimit'
    | 'effectiveDefaultListLimit'
    | 'effectiveMaxListLimit'
    | 'effectiveDefaultFeaturesLimit'
    | 'effectiveMaxFeaturesLimit'

// Matches storage.SettingsPatch — every field optional, an omitted one left
// unchanged server-side.
export type SettingsPatch = Partial<Omit<Settings, SettingsReadOnlyKey>>

// LimitField is a per-table-overridable field whose effective value is a
// default/max pair (maxRowLimit, maxFeaturesLimit) — override is the
// operator's own value for this table, null when inherited from the global
// row.
export interface LimitField {
    override: number | null
    effectiveDefault: number
    effectiveMax: number
}

// OverrideField is a per-table-overridable field with a single effective
// value (maxSimplifyTolerance, maxPrecision, tileCacheMaxAgeSec,
// coordinateDecimals) — override is null when inherited.
export interface OverrideField {
    override: number | null
    effective: number
}

// Matches the tableSettingsDTO the server returns from GET/PATCH/DELETE
// /tables/{tableId}/settings and GET /settings/tables: a table's departure
// from the global row, field by field. maxRowLimit applies to every table;
// the rest are present only on a geo table (isGeo) — absent, not disabled,
// on a table the fields could never apply to.
export interface TableSettings {
    tableId: string
    isGeo: boolean
    maxRowLimit: LimitField
    // maxFeaturesLimit, like maxRowLimit, pairs a default and a max
    // (internal/settings.Limits) — the only two per-table fields that do.
    maxFeaturesLimit?: LimitField
    maxSimplifyTolerance?: OverrideField
    maxPrecision?: OverrideField
    tileCacheMaxAgeSec?: OverrideField
    coordinateDecimals?: OverrideField
    updatedAt: string
}

// The fields a table's settings can carry an override for, one per
// TableSettings field but the read-only tableId/isGeo/updatedAt.
export type TableSettingsFieldKey =
    | 'maxRowLimit'
    | 'maxFeaturesLimit'
    | 'maxSimplifyTolerance'
    | 'maxPrecision'
    | 'tileCacheMaxAgeSec'
    | 'coordinateDecimals'

// A PATCH body for a table's settings: a key absent leaves that override
// unchanged, present with null clears it back to "inherit," present with a
// number sets it.
export type TableSettingsPatch = Partial<Record<TableSettingsFieldKey, number | null>>

export interface TableSettingsOverridesPage {
    tables: TableSettings[]
}
