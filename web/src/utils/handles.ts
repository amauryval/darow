import { readPaths, type VertexRef } from "./vertices";

// The dots the user grabs on the map: one per vertex, plus one at the
// middle of every segment — dragging a midpoint is how a vertex is added,
// the same gesture as in every desktop GIS.
//
// They travel as an ordinary GeoJSON source: MapLibre hit-tests a circle
// layer far better than a DOM marker per vertex, and a parcel with three
// hundred vertices would be three hundred DOM nodes.
export type HandleRole = "vertex" | "midpoint";

// A feature property is a primitive, so the vertex address travels as text
// and is read back through handleRef.
function properties(role: HandleRole, ref: VertexRef): Record<string, string | number> {
  return { role, path: ref.path.join(","), index: ref.index };
}

function point(position: GeoJSON.Position, role: HandleRole, ref: VertexRef): GeoJSON.Feature<GeoJSON.Point> {
  return { type: "Feature", geometry: { type: "Point", coordinates: position }, properties: properties(role, ref) };
}

function middle(a: GeoJSON.Position, b: GeoJSON.Position): GeoJSON.Position {
  return [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
}

export function handleFeatures(geometry: GeoJSON.Geometry | null): GeoJSON.FeatureCollection<GeoJSON.Point> {
  const features: Array<GeoJSON.Feature<GeoJSON.Point>> = [];
  for (const { path, coordinates, closed } of geometry ? readPaths(geometry) : []) {
    // A closed ring repeats its first vertex last; that duplicate gets no
    // handle of its own, or the user would drag one end of a boundary that
    // has no ends.
    const last = closed ? coordinates.length - 1 : coordinates.length;
    for (let index = 0; index < last; index += 1) features.push(point(coordinates[index], "vertex", { path, index }));
    for (let index = 1; index < coordinates.length; index += 1) {
      features.push(point(middle(coordinates[index - 1], coordinates[index]), "midpoint", { path, index }));
    }
  }
  return { type: "FeatureCollection", features };
}

// The address a handle carries, or null when the properties are not a
// handle's — a click landing on another layer.
export function handleRef(props: GeoJSON.GeoJsonProperties): { role: HandleRole; ref: VertexRef } | null {
  if (!props) return null;
  const { role, path, index } = props;
  if ((role !== "vertex" && role !== "midpoint") || typeof path !== "string" || typeof index !== "number") return null;
  return { role, ref: { path: path === "" ? [] : path.split(",").map(Number), index } };
}
