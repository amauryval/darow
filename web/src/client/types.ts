export interface Metadata {
  tableId: string;
  physicalName: string;
  tableName: string;
  isGeo: boolean;
  createdAt: string;
  updatedAt: string;
  geometryColumn?: string;
  rowCount: number;
  bbox?: string;
  srid: number;
}

interface SourceLayer {
  name: string;
  loaded: boolean;
  tableId?: string;
}

export interface Source {
  name: string;
  ext: string;
  sizeBytes: number;
  modTime: string;
  loaded: boolean;
  tableId?: string;
  layers?: SourceLayer[];
}

export interface SourceCRS {
  srid: number | null;
  authName: string;
  // Whether the source carries a geometry field at all. It decides whether a
  // CRS has to be asked for: a file of plain columns has nothing to project,
  // and the server takes it with no srid.
  hasGeometry: boolean;
}

export interface TablesPage {
  tables: Metadata[];
  total: number;
  limit: number;
  offset: number;
}

export interface SourcesPage {
  sources: Source[];
  total: number;
  limit: number;
  offset: number;
}

export type RowValue = string | number | boolean | null;

export interface Row {
  pkey: string;
  [column: string]: RowValue;
}

// A row's geometry, as served by the per-row geometry route: GeoJSON to
// draw it, WKT to read it. Both at full resolution, in lon/lat.
export interface RowGeometry {
  pkey: string;
  geometry: GeoJSON.Geometry | null;
  wkt: string;
}

export interface RowsPage {
  rows: Row[];
  total: number;
  limit: number;
  offset: number;
}

export interface ColumnSchema {
  name: string;
  type: string;
}

export interface SchemaResponse {
  columns: ColumnSchema[];
}

// Matches storage.FilterCondition — a node is either a leaf
// (column/operator/value) or a boolean group (bool + conditions), never
// both. Column and operator are validated server-side against the table's
// real schema and a fixed operator whitelist, so no enum is enforced
// client-side.
export interface FilterCondition {
  column?: string;
  operator?: string;
  value?: RowValue;
  // Negates the node, leaf or group alike.
  not?: boolean;
  bool?: "AND" | "OR";
  conditions?: FilterCondition[];
}

// One column a guided derivation computes rather than carries over: a free
// SQL scalar expression over the source table's columns, under a chosen
// name. The expression is validated server-side (DuckDB's own parser), so
// nothing checks it here beyond it being non-empty.
export interface ComputedColumn {
  alias: string;
  expression: string;
}

// Matches service.DeriveSpec — a derivation from one source table, described
// structurally instead of as SQL. Every field but newTableName is optional:
// an empty spec derives the whole source table. The SQL is assembled
// server-side, which is why no query string appears here.
export interface DeriveSpec {
  newTableName: string;
  columns?: string[];
  computed?: ComputedColumn[];
  conditions?: FilterCondition[];
}

// A filter saved against a table, server-side. Its conditions are the same
// tree an ad-hoc filter posts, validated against the table's real schema at
// save time — so a stored filter is always applicable. Deleting the table
// deletes its filters.
export interface SavedFilter {
  filterId: string;
  tableId: string;
  name: string;
  conditions: FilterCondition[];
  createdAt: string;
  updatedAt: string;
}

export interface SavedFiltersPage {
  filters: SavedFilter[];
  total: number;
}

export interface ColumnDTO {
  name: string;
  type: string;
}

export interface ErrorResponse {
  error: string;
  message: string;
  code: number;
  requestId: string;
}
