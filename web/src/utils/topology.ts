import { area } from "@turf/area";
import { booleanIntersects } from "@turf/boolean-intersects";
import { difference } from "@turf/difference";
import { flatten } from "@turf/flatten";
import { union } from "@turf/union";

// Shared-boundary editing, as set operations rather than as vertex
// bookkeeping. Reshaping a parcel does two things to what surrounds it: it
// takes area (where the new shape covers what used to be a neighbour's) and
// it frees area (where the old shape no longer reaches). A neighbour
// follows the edit by losing the first and gaining the second — which turf
// answers with `difference` and `union`, whatever the vertices did.
//
// The gain is restricted to the freed pieces that touch the neighbour: a
// parcel two streets away has nothing to claim.
type Areal = GeoJSON.Polygon | GeoJSON.MultiPolygon;

export function isAreal(geometry: GeoJSON.Geometry): geometry is Areal {
  return geometry.type === "Polygon" || geometry.type === "MultiPolygon";
}

function feature(geometry: Areal): GeoJSON.Feature<Areal> {
  return { type: "Feature", geometry, properties: {} };
}

function collection(...geometries: Areal[]): GeoJSON.FeatureCollection<Areal> {
  return { type: "FeatureCollection", features: geometries.map(feature) };
}

function minus(a: Areal, b: Areal): Areal | null {
  return difference(collection(a, b))?.geometry ?? null;
}

function plus(a: Areal, b: Areal): Areal | null {
  return union(collection(a, b))?.geometry ?? null;
}

// Clipping rebuilds coordinates, so two runs of the same shape rarely
// compare equal literally. What matters is whether any area changed hands —
// under a square millimeter is the clipper's own noise, not an edit.
const NOISE_SQUARE_METERS = 1e-6;

function unchanged(a: Areal, b: Areal): boolean {
  const left = minus(a, b);
  const right = minus(b, a);
  return (
    (!left || area(feature(left)) < NOISE_SQUARE_METERS) && (!right || area(feature(right)) < NOISE_SQUARE_METERS)
  );
}

// What the edit freed, in pieces: a single reshape can retreat in several
// places at once, and each piece goes to whoever it touches.
export function freedPieces(before: Areal, after: Areal): GeoJSON.Polygon[] {
  const freed = minus(before, after);
  return freed ? flatten(feature(freed)).features.flatMap((f) => (f.geometry ? [f.geometry] : [])) : [];
}

// The neighbour as the edit leaves it, or null when the edit did not reach
// it — nothing shared, nothing to rewrite.
export function reshapeNeighbour(neighbour: Areal, after: Areal, freed: GeoJSON.Polygon[]): Areal | null {
  const overlaps = booleanIntersects(feature(neighbour), feature(after));
  const claimed = freed.filter((piece) => booleanIntersects(feature(piece), feature(neighbour)));
  if (!overlaps && claimed.length === 0) return null;

  // Cut first, fill second: the edited shape always wins the area it now
  // covers, so a neighbour can never be given back a piece the edit took.
  let result: Areal | null = overlaps ? minus(neighbour, after) : neighbour;
  for (const piece of claimed) {
    if (!result) break;
    result = plus(result, piece);
  }
  // A cut that consumed the neighbour whole is not a rewrite, it is a
  // deletion — and deleting a row is never a side effect of an edit. A
  // neighbour that only ever touched the edited limit comes back identical
  // and is left alone: writing it would be a write with nothing in it.
  return result && !unchanged(result, neighbour) ? result : null;
}
