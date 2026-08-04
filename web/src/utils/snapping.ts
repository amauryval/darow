import { distance } from "@turf/distance";
import { nearestPointOnLine } from "@turf/nearest-point-on-line";
import { readPaths } from "./vertices";

// Snapping: the placed vertex is pulled onto what is already there, so two
// features end up sharing the exact same coordinate rather than two
// coordinates a hair apart. Vertices win over segments — landing on a
// corner is almost always what was meant when both are within reach.
//
// Distances are turf's, in meters on the sphere; the tolerance is a
// screen-space one converted by the caller, so it stays the same handful of
// pixels whatever the zoom.
const DEGREE_METERS = 111_320;

function point(position: GeoJSON.Position): GeoJSON.Feature<GeoJSON.Point> {
  return { type: "Feature", geometry: { type: "Point", coordinates: position }, properties: {} };
}

function line(coordinates: GeoJSON.Position[]): GeoJSON.Feature<GeoJSON.LineString> {
  return { type: "Feature", geometry: { type: "LineString", coordinates }, properties: {} };
}

// Rules out the far-away candidates on plain coordinates before paying for
// a haversine: a drag fires this on every mouse move, over every loaded
// neighbour.
function near(a: GeoJSON.Position, b: GeoJSON.Position, meters: number): boolean {
  const degrees = meters / DEGREE_METERS;
  return Math.abs(a[0] - b[0]) < degrees * 4 && Math.abs(a[1] - b[1]) < degrees;
}

export function snapPosition(
  position: GeoJSON.Position,
  references: GeoJSON.Geometry[],
  tolerance: number,
): GeoJSON.Position {
  const from = point(position);
  let best: { position: GeoJSON.Position; distance: number } | null = null;

  for (const reference of references) {
    for (const { coordinates } of readPaths(reference)) {
      for (const candidate of coordinates) {
        if (!near(position, candidate, tolerance)) continue;
        const meters = distance(from, point(candidate), { units: "meters" });
        if (meters <= tolerance && (!best || meters < best.distance)) best = { position: candidate, distance: meters };
      }
    }
  }
  if (best) return best.position;

  for (const reference of references) {
    for (const { coordinates } of readPaths(reference)) {
      if (coordinates.length < 2) continue;
      const nearest = nearestPointOnLine(line(coordinates), from, { units: "meters" });
      const meters = nearest.properties.dist;
      if (typeof meters !== "number" || meters > tolerance) continue;
      if (!best || meters < best.distance) best = { position: nearest.geometry.coordinates, distance: meters };
    }
  }
  return best ? best.position : position;
}
