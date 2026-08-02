import type { Source } from "../client/types";

// The tree the sources table renders: a single-layer file is one row, a
// multi-layer one (GeoPackage) is a parent row expanded over its layers —
// only a layer row carries a load/view action, the parent only a deletion.
export interface SourceRow {
  name: string;
  sizeBytes?: number;
  loaded: boolean;
  tableId?: string;
  layerCount?: number;
  source: Source;
  layerName?: string;
  subRows?: SourceRow[];
}

export function toSourceRows(sources: Source[]): SourceRow[] {
  return sources.map((source) => {
    if (source.layers && source.layers.length > 0) {
      return {
        name: source.name,
        loaded: false,
        sizeBytes: source.sizeBytes,
        layerCount: source.layers.length,
        source,
        subRows: source.layers.map((layer) => ({
          name: layer.name,
          loaded: layer.loaded,
          tableId: layer.tableId,
          source,
          layerName: layer.name,
        })),
      };
    }
    return { name: source.name, sizeBytes: source.sizeBytes, loaded: source.loaded, tableId: source.tableId, source };
  });
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} Ko`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} Mo`;
}
