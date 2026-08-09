export interface Diagnostics {
    databaseSizeBytes: number
    memoryUsageBytes: number
    temporaryFilesBytes: number
    extensions: string[]
    // duckdb.maxSizeBytes from config.yaml — 0 means unlimited.
    maxSizeBytes: number
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
// The three default*Limit fields are one page size per family of pages: how
// many items the server returns when a request asks for no limit, and the
// most it returns when a request asks for more. They are stored, never
// derived, so what is read here is what the server applies.
export interface Settings {
    defaultRowLimit: SettingsField
    defaultListLimit: SettingsField
    defaultFeaturesLimit: SettingsField
    maxSimplifyTolerance: SettingsField
    maxPrecision: SettingsField
    tileCacheMaxAgeSec: SettingsField
    cacheTTLSec: SettingsField
    coordinateDecimals: SettingsField
    updatedAt: string
}

// FieldBounds is what the server enforces on a setting, published beside it:
// the unit its value is expressed in, and the range internal/settings.Validate
// refuses outside of. A bound the field does not have is absent — never 0
// standing in for "none". minimum is inclusive, exclusiveMinimum is not, and
// a field states one of the two, never both.
export interface FieldBounds {
    unit: string
    minimum?: number
    exclusiveMinimum?: number
    maximum?: number
}

// One setting as the read routes serve it: the value applied and what
// constrains it, in a single object.
export interface SettingsField extends FieldBounds {
    value: number
}

// The read-only half of Settings: derived server-side, never sent back.
export type SettingsReadOnlyKey = 'updatedAt'

// The fields a PATCH may carry, one per editable field of Settings.
export type SettingsFieldKey = keyof Omit<Settings, SettingsReadOnlyKey>

// Matches storage.SettingsPatch — every field optional, an omitted one left
// unchanged server-side. A patch carries bare numbers: the bounds travel
// with the read, never back with the write.
export type SettingsPatch = Partial<Record<SettingsFieldKey, number>>

// OverrideField is a per-table-overridable field: override is the operator's
// own value for this table, null when inherited from the global row, and
// effective is the value the server applies to it either way. Its bounds are
// the global row's own — an override is validated against them too.
export interface OverrideField extends FieldBounds {
    override: number | null
    effective: number
}

// Matches the tableSettingsDTO the server returns from GET/PATCH/DELETE
// /tables/{tableId}/settings and GET /settings/tables: a table's departure
// from the global row, field by field. defaultRowLimit applies to every
// table; the rest are present only on a geo table (isGeo) — absent, not
// disabled, on a table the fields could never apply to.
export interface TableSettings {
    tableId: string
    isGeo: boolean
    defaultRowLimit: OverrideField
    defaultFeaturesLimit?: OverrideField
    maxSimplifyTolerance?: OverrideField
    maxPrecision?: OverrideField
    tileCacheMaxAgeSec?: OverrideField
    coordinateDecimals?: OverrideField
    updatedAt: string
}

// The fields a table's settings can carry an override for, one per
// TableSettings field but the read-only tableId/isGeo/updatedAt.
export type TableSettingsFieldKey =
    | 'defaultRowLimit'
    | 'defaultFeaturesLimit'
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
