import { MercatorCoordinate } from "maplibre-gl";

export interface Tile {
  z: number;
  x: number;
  y: number;
}

const MAX_MERCATOR_LAT = 85.05112878;

function lonLatToTile(lon: number, lat: number, z: number): { x: number; y: number } {
  const { x, y } = MercatorCoordinate.fromLngLat({ lng: lon, lat });
  return { x: Math.floor(x * 2 ** z), y: Math.floor(y * 2 ** z) };
}

export function tileKey(tile: Tile): string {
  return `${tile.z}/${tile.x}/${tile.y}`;
}

// Same formula as tileSimplifyTolerance in internal/storage/duckdb/mvt.go:
// a Web Mercator tolerance (meters) collapsing to roughly one MVT extent
// unit at zoom, so a GeoJSON tile carries no more coordinate detail than its
// MVT counterpart would render — the main lever against unbounded RAM use at
// low zoom, where a tile's geometries would otherwise ship full-resolution.
export function tileSimplifyTolerance(zoom: number): number {
  const webMercatorCircumference = 40075016;
  const mvtExtent = 4096;
  const tolerance = webMercatorCircumference / (mvtExtent * 2 ** zoom);
  return Math.max(tolerance, 0.25);
}

// Standard slippy-map XYZ grid: tiles covering a lon/lat bbox at zoom z.
export function tilesCovering(bbox: [number, number, number, number], z: number): Tile[] {
  const [minLon, minLat, maxLon, maxLat] = bbox;
  const clampedMinLat = Math.max(minLat, -MAX_MERCATOR_LAT);
  const clampedMaxLat = Math.min(maxLat, MAX_MERCATOR_LAT);
  const maxTileIndex = 2 ** z - 1;

  const topLeft = lonLatToTile(minLon, clampedMaxLat, z);
  const bottomRight = lonLatToTile(maxLon, clampedMinLat, z);

  const minX = Math.max(0, Math.min(maxTileIndex, topLeft.x));
  const maxX = Math.max(0, Math.min(maxTileIndex, bottomRight.x));
  const minY = Math.max(0, Math.min(maxTileIndex, topLeft.y));
  const maxY = Math.max(0, Math.min(maxTileIndex, bottomRight.y));

  const tiles: Tile[] = [];
  for (let x = minX; x <= maxX; x++) {
    for (let y = minY; y <= maxY; y++) {
      tiles.push({ x, y, z });
    }
  }
  return tiles;
}
