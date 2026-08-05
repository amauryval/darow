import { createXYZ } from "ol/tilegrid";
import type TileGrid from "ol/tilegrid/TileGrid";

// Same formula as tileSimplifyTolerance in internal/storage/duckdb/data/geo.go:
// a Web Mercator tolerance (meters) collapsing to roughly one MVT extent
// unit at zoom, so a GeoJSON tile carries no more coordinate detail than its
// MVT counterpart would render — the main lever against unbounded RAM use at
// low zoom, where a tile's geometries would otherwise ship full-resolution.
//
// The grid itself is OpenLayers' (`createXYZ`): what is left here is the one
// thing the server and the client have to agree on and no library knows.
export function tileSimplifyTolerance(zoom: number): number {
  const webMercatorCircumference = 40075016;
  const mvtExtent = 4096;
  const tolerance = webMercatorCircumference / (mvtExtent * 2 ** zoom);
  return Math.max(tolerance, 0.25);
}

// The grid both tile endpoints are cut along, built once so the two render
// modes cannot drift apart.
//
// It has to be stated, not defaulted: a vector-tile source left to itself
// uses 512-pixel tiles, which asks for a different z for the same view than
// the 256-pixel XYZ grid the server's zoom-based simplification — and
// tileSimplifyTolerance above — are written against. The two modes would
// then request different tiles for the same map, and the tolerance the
// server applies would not be the one the zoom means here.
export function xyzGrid(): TileGrid {
  return createXYZ({ tileSize: 256 });
}
