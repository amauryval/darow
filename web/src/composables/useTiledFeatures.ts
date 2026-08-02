import { getTileFeatures } from "../client/geo";
import type { OgcFeature } from "../client/features";
import type { FilterCondition } from "../client/types";
import { tileKey, tileSimplifyTolerance, tilesCovering } from "../utils/tileGrid";

const TILE_FEATURE_PAGE_SIZE = 10_000;

// Fetches every feature of a single tile, paging with limit/offset as long
// as a full page keeps coming back — a tile covering a dense area can hold
// more features than one page, and dropping the rest silently would make
// the map look like it's missing data at low zoom (large tiles). Simplified
// to the tile's own pixel resolution (tileSimplifyTolerance), same as the
// MVT tile endpoint, so a low-zoom tile doesn't ship full-detail coordinates
// for geometry no pixel on screen could show anyway.
// complete is false when a page request failed partway through — the
// caller must not treat the features gathered so far as the tile's whole
// content (§8.6: "a silent partial result is a lie"), unlike a short page,
// which is the normal, expected end of the tile's data.
async function fetchAllTileFeatures(
  tableId: string,
  z: number,
  x: number,
  y: number,
  filter: FilterCondition[],
): Promise<{ features: OgcFeature[]; complete: boolean }> {
  const features: OgcFeature[] = [];
  let offset = 0;
  for (;;) {
    let collection;
    try {
      collection = await getTileFeatures(tableId, z, x, y, {
        limit: TILE_FEATURE_PAGE_SIZE,
        offset,
        simplify: tileSimplifyTolerance(z),
        filter,
      });
    } catch {
      return { features, complete: false };
    }
    features.push(...collection.features);
    if (collection.features.length < TILE_FEATURE_PAGE_SIZE) break;
    offset += TILE_FEATURE_PAGE_SIZE;
  }
  return { features, complete: true };
}

// Splits WFS/OGC-Features loading into the same XYZ tile grid the MVT layer
// uses, instead of one query over the whole viewport. Nothing accumulates
// across the whole session — lastTiles only ever holds the tiles from the
// previous call, replaced wholesale on every call — but a tile that's still
// visible after a small pan/zoom (most of them, usually) is reused instead
// of re-fetched, saving the HTTP round trip for exactly the tiles that
// didn't change.
export function useTiledFeatures() {
  let lastTiles = new Map<string, OgcFeature[]>();
  let lastFilterKey = "";

  // onProgress, if given, is called with the merged feature set after every
  // tile lands — a caller can render each call to make the map fill in tile
  // by tile instead of blocking on the slowest tile in view.
  //
  // The returned complete flag is false as soon as one tile's fetch failed
  // partway through: the caller still gets every feature that did load
  // (rendering something beats rendering nothing), but must not present it
  // as the whole picture. A failed tile is deliberately left out of the
  // cache, so the next call (the next pan/zoom, or a caller-triggered
  // retry) fetches it again instead of being stuck with the partial result.
  async function loadVisible(
    tableId: string,
    viewBbox: [number, number, number, number],
    zoom: number,
    filter: FilterCondition[],
    onProgress?: (features: OgcFeature[]) => void,
  ): Promise<{ features: OgcFeature[]; complete: boolean }> {
    const filterKey = JSON.stringify(filter);
    // A tile cached under a different filter no longer reflects what the
    // current filter would return, so a filter change invalidates the whole
    // cache rather than just growing it under a new key.
    if (filterKey !== lastFilterKey) {
      lastTiles = new Map();
      lastFilterKey = filterKey;
    }

    const tiles = tilesCovering(viewBbox, zoom);
    const currentTiles = new Map<string, OgcFeature[]>();
    // A geometry straddling a tile boundary is returned by every tile it
    // intersects, so features are merged by id rather than concatenated.
    const byId = new Map<string, OgcFeature>();
    let complete = true;

    await Promise.all(
      tiles.map(async (tile) => {
        const key = tileKey(tile);
        const cached = lastTiles.get(key);
        let features: OgcFeature[];
        if (cached) {
          features = cached;
        } else {
          const result = await fetchAllTileFeatures(tableId, tile.z, tile.x, tile.y, filter);
          features = result.features;
          if (result.complete) {
            currentTiles.set(key, features);
          } else {
            complete = false;
          }
        }
        for (const feature of features) byId.set(feature.id, feature);
        onProgress?.(Array.from(byId.values()));
      }),
    );

    lastTiles = currentTiles;
    return { features: Array.from(byId.values()), complete };
  }

  return { loadVisible };
}
