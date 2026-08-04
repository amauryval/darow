import { absoluteApiUrl, get, post } from "./http";
import type { OgcFeatureCollection } from "./features";
import type { FilterCondition } from "./types";

// filter, when non-empty, is JSON-encoded the same way GetItems/
// getTileFeatures take it — the MVT tile endpoint applies it server-side
// (règle 5), so a client-controlled filter narrows what a tile renders
// exactly like it narrows a GeoJSON/OGC-Features query.
// version, when given, is appended as a plain query parameter the server
// ignores: the tile's content is already keyed on the table's UpdatedAt
// server-side, but MapLibre keeps its own copy of a tile per URL, so a
// write has to change the URL for the map to ask again.
export function tileUrl(tableId: string, filter: FilterCondition[] = [], version = 0): string {
  const params = new URLSearchParams();
  if (filter.length > 0) params.set("filter", JSON.stringify(filter));
  if (version > 0) params.set("v", String(version));
  const qs = params.toString();
  return `${absoluteApiUrl(`/tables/${tableId}/tiles/{z}/{x}/{y}`)}${qs ? `?${qs}` : ""}`;
}

export interface TileFeatureQueryOptions {
  limit?: number;
  offset?: number;
  simplify?: number;
  filter?: FilterCondition[];
}

// Same z/x/y grid as tileUrl's MVT tiles, but as a GeoJSON FeatureCollection —
// the tile envelope is computed server-side (ST_TileEnvelope), so a client
// chunking its GeoJSON loading by tile never has to derive a tile's bbox
// itself. limit/offset page a single tile's results, same contract as OGC
// API - Features' items endpoint; simplify is the same tolerance override
// the MVT tile endpoint gets implicitly from its own zoom-based formula.
export function getTileFeatures(
  tableId: string,
  z: number,
  x: number,
  y: number,
  opts: TileFeatureQueryOptions = {},
): Promise<OgcFeatureCollection> {
  const params = new URLSearchParams();
  if (opts.limit !== undefined) params.set("limit", String(opts.limit));
  if (opts.offset !== undefined) params.set("offset", String(opts.offset));
  if (opts.simplify !== undefined) params.set("simplify", String(opts.simplify));
  if (opts.filter !== undefined && opts.filter.length > 0) params.set("filter", JSON.stringify(opts.filter));
  const qs = params.toString();
  return get(`/tables/${tableId}/tiles/${z}/${x}/${y}/features${qs ? `?${qs}` : ""}`);
}

export function filterByBbox(tableId: string, bbox: [number, number, number, number]): Promise<{ pkeys: string[] }> {
  return post(`/tables/${tableId}/bbox`, { bbox });
}

// bbox is "minX,minY,maxX,maxY" in OGC:CRS84, same format as Metadata.bbox —
// empty when no row matches. Used to re-center the map on a filter's
// results instead of the whole table's extent.
export function getFilteredExtent(tableId: string, filter: FilterCondition[] = []): Promise<{ bbox: string }> {
  const qs = filter.length > 0 ? `?filter=${encodeURIComponent(JSON.stringify(filter))}` : "";
  return get(`/tables/${tableId}/filtered-extent${qs}`);
}

// The features around a geometry: everything whose own geometry falls
// within buffer meters of it, minus the rows named by exclude. This is what
// an edit session loads to snap to its neighbours and to carry a shared
// boundary along when it moves.
export function getNeighbours(
  tableId: string,
  geometry: GeoJSON.Geometry,
  buffer: number,
  exclude: string[] = [],
  limit?: number,
): Promise<OgcFeatureCollection> {
  return post(`/tables/${tableId}/neighbours`, { geometry, buffer, exclude, limit });
}

// Distinct geometry types found in the table's geometry column — computed
// on demand server-side, never cached (a column can mix types).
export function getGeometryTypes(tableId: string): Promise<{ types: string[] }> {
  return get(`/tables/${tableId}/geometry-types`);
}
