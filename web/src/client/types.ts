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

export interface SourceLayer {
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
}

export type RowValue = string | number | boolean | null;

export interface Row {
  pkey: string;
  [column: string]: RowValue;
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

export interface ErrorResponse {
  error: string;
  message: string;
  code: number;
}
