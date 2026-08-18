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
    // The workDir file the table was imported from, absent for a derived one.
    sourceName?: string
    sourceLayer?: string
}

// One entry of the provenance index: which table came from which source file.
// A derived table has no entry at all.
export interface TableProvenance {
    tableId: string
    tableName: string
    sourceName: string
    sourceLayer?: string
}

// A file found in workDir. It says nothing of the tables loaded from it: a
// load copies the data, and the table lives on its own from then on.
export interface Source {
    name: string
    ext: string
    sizeBytes: number
    modTime: string
    layers?: string[]
}

// Matches deletedDTO: what a delete route confirms happened. id is the deleted
// object's identifier (a table/source name, a row's pkey); name only when it
// differs from id; rowsRemoved only on a row-level delete.
export interface Deleted {
    id: string
    name?: string
    rowsRemoved?: number
}

// Matches uploadResultDTO — POST /sources/upload's response: the freshly
// scanned source, layers included. Loading it is a separate, explicit step.
export interface UploadResult {
    source?: Source
}

// Matches sourceLimitsDTO of /sources/limits: what an upload has to satisfy,
// read from the running deployment's config.yaml rather than assumed.
export interface SourceLimits {
    maxSizeBytes: number
    allowedExtensions: string[]
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
// draw it, WKT to read it. crs is what they are actually expressed in — the
// requested EPSG, else the table's own, else lon/lat — and is displayed
// rather than assumed.
export interface RowGeometry {
    pkey: string
    geometry: GeoJSON.Geometry | null
    wkt: string
    crs: string
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

// Matches storage.FilterCondition — a node is either a leaf (column/operator/
// value) or a boolean group (bool + conditions), never both. Both are
// validated server-side, so no enum is enforced client-side.
export interface FilterCondition {
    column?: string
    operator?: string
    value?: RowValue
    // Negates the node, leaf or group alike.
    not?: boolean
    bool?: 'AND' | 'OR'
    conditions?: FilterCondition[]
}

// One column a guided derivation computes rather than carries over: a free SQL
// scalar expression over the source table, under a chosen name. It is
// validated server-side, so nothing checks it here beyond being non-empty.
export interface ComputedColumn {
    alias: string
    expression: string
}

// Matches service.DeriveSpec — a derivation from one source table, described
// structurally instead of as SQL. Every field but newTableName is optional.
// The SQL is assembled server-side, hence no query string here.
export interface DeriveSpec {
    newTableName: string
    columns?: string[]
    computed?: ComputedColumn[]
    conditions?: FilterCondition[]
}

// One column a guided aggregation computes: a free SQL aggregate expression
// (`COUNT(*)`, `SUM(height)`) under a chosen alias — the aggregation
// counterpart of ComputedColumn.
export interface AggregateColumn {
    alias: string
    expression: string
}

// Matches service.AggregateSpec — a GROUP BY derivation from one source table,
// distinct from DeriveSpec: an empty groupBy aggregates the whole table into
// one row, never "every column".
export interface AggregateSpec {
    newTableName: string
    groupBy?: string[]
    aggregates?: AggregateColumn[]
    conditions?: FilterCondition[]
}

// A filter saved against a table, server-side. Its conditions are the tree an
// ad-hoc filter posts, validated against the real schema at save time — so a
// stored filter is always applicable. Deleting the table deletes its filters.
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

// Matches storage.Settings — every server parameter an operator retunes while
// the process is serving. What stays in Config (port, DuckDB path, workDir,
// the memory guards) is absent here, so it has no place on a settings screen.
// The three default*Limit fields are one stored page size per family of pages.
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

// What the server enforces on a setting, published beside it: its unit and the
// range internal/settings.Validate refuses outside of. An absent bound means
// none, never 0; a field states minimum or exclusiveMinimum, never both.
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

// A per-table-overridable field: override is this table's own value, null when
// inherited from the global row, and effective is what the server applies
// either way. Its bounds are the global row's — an override is checked too.
export interface OverrideField extends FieldBounds {
    override: number | null
    effective: number
}

// Matches the tableSettingsDTO of /tables/{tableId}/settings: a table's
// departure from the global row, field by field. defaultRowLimit applies to
// every table; the rest are present only on a geo table — absent, not disabled.
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

// ExportFormat mirrors storage.ExportFormat: csv carries no geometry, the
// other two do and are the only ones an output CRS applies to (back §5.6).
export type ExportFormat = 'csv' | 'geojson' | 'parquet'

// Matches service.Operand: a reference to another column of the same row, or
// a literal, inside a concatenation or an arithmetic operation. Exactly one
// of column/literal is meaningful, selected by kind.
export interface Operand {
    kind: 'column' | 'literal'
    column?: string
    literal?: RowValue
}

// Matches service.ArithmeticOp: a whitelisted binary operation ('+' '-' '*'
// '/') over two operands the server checks numeric against the schema.
export interface ArithmeticOp {
    left: Operand
    right: Operand
    operator: string
}

export type AssignmentKind = 'constant' | 'concat' | 'arithmetic' | 'expression'

// Matches service.Assignment: what a column transform's case (or its
// default) writes. Exactly one field is meaningful, selected by kind; the
// server renders it, no SQL is ever assembled here.
export interface Assignment {
    kind: AssignmentKind
    constant?: RowValue
    concat?: Operand[]
    arithmetic?: ArithmeticOp
    expression?: string
}

// Matches service.TransformCase: one arm of a column transform's CASE WHEN —
// the rows it applies to (never empty) and the value they receive.
export interface TransformCase {
    conditions: FilterCondition[]
    assignment: Assignment
}

// Matches service.TransformSpec, the body of the transform and its preview
// route: an in-place column rewrite described structurally, over the rows
// conditions names (all rows when empty).
export interface TransformSpec {
    cases: TransformCase[]
    default: Assignment
    conditions?: FilterCondition[]
}

export interface TransformResult {
    written: number
}

// Matches storage.TransformSample: one row's current and would-be value for
// a column transform preview, computed without writing anything.
export interface TransformSample {
    pkey: string
    before: RowValue
    after: RowValue
}

export interface TransformPreview {
    matched: number
    sample: TransformSample[]
}

// Matches goauth's own config and session DTOs — the only two here that are
// NOT camelCase: AuthSession.user is a bare Go struct with no JSON tags, so
// its fields keep their Go casing. Renaming a key is a silent `undefined`.

// All three empty strings means demo mode: no provider is configured, so
// there is no login control to show at all (auth/oidc.ts skips building a
// UserManager entirely).
export interface AuthConfig {
    issuer_url: string
    client_id: string
    scopes: string
}

export interface AuthUser {
    Provider: string
    ID: string
    Roles: string[]
}

export interface AuthSession {
    logged_in: boolean
    authorized: boolean
    roles?: string[]
    user?: AuthUser
}
