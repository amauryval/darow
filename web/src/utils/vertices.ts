// Addressing a vertex inside a geometry, whatever its type. A path names
// the coordinate list — `[]` for a point or a line, `[ring]` for a polygon,
// `[polygon, ring]` for a multipolygon — and an index names the vertex in
// it. Every edit on the map is one of three moves on that address: move,
// insert, remove.
export interface VertexRef {
  path: number[];
  index: number;
}

interface CoordinatePath {
  path: number[];
  coordinates: GeoJSON.Position[];
  // A ring repeats its first vertex last. That duplicate is never shown as
  // a handle and never edited on its own — it follows the first.
  closed: boolean;
}

// The shapes the map editor draws. A geometry read from a row may be any of
// the six simple types; these are the three a session creates.
export type DrawShape = "Point" | "LineString" | "Polygon";

// The geometry a shape being drawn amounts to so far, or null while it is
// still too short to be one — two vertices for a line, three for a
// polygon. The ring is closed here and nowhere else: what is collected on
// the map is the list of vertices the user clicked, never the duplicate.
export function draftGeometry(shape: DrawShape, positions: GeoJSON.Position[]): GeoJSON.Geometry | null {
  switch (shape) {
    case "Point":
      return positions.length >= 1 ? { type: "Point", coordinates: positions[0] } : null;
    case "LineString":
      return positions.length >= 2 ? { type: "LineString", coordinates: positions } : null;
    case "Polygon":
      return positions.length >= 3 ? { type: "Polygon", coordinates: [[...positions, positions[0]]] } : null;
  }
}

// The clicks a drawn geometry amounts to — draftGeometry read backwards.
// A vertex dragged or removed while the shape is still being drawn has to
// come back into that list, or the next click would rebuild the shape from
// where the vertices used to be.
export function draftPositions(geometry: GeoJSON.Geometry): GeoJSON.Position[] {
  switch (geometry.type) {
    case "Point":
      return [geometry.coordinates];
    case "LineString":
      return geometry.coordinates;
    case "Polygon":
      return geometry.coordinates[0].slice(0, -1);
    default:
      return [];
  }
}

export function readPaths(geometry: GeoJSON.Geometry): CoordinatePath[] {
  switch (geometry.type) {
    case "Point":
      return [{ path: [], coordinates: [geometry.coordinates], closed: false }];
    case "MultiPoint":
    case "LineString":
      return [{ path: [], coordinates: geometry.coordinates, closed: false }];
    case "MultiLineString":
      return geometry.coordinates.map((coordinates, i) => ({ path: [i], coordinates, closed: false }));
    case "Polygon":
      return geometry.coordinates.map((coordinates, i) => ({ path: [i], coordinates, closed: true }));
    case "MultiPolygon":
      return geometry.coordinates.flatMap((rings, i) =>
        rings.map((coordinates, j) => ({ path: [i, j], coordinates, closed: true })),
      );
    case "GeometryCollection":
      return [];
  }
}

function writePath(geometry: GeoJSON.Geometry, path: number[], coordinates: GeoJSON.Position[]): GeoJSON.Geometry {
  switch (geometry.type) {
    case "Point":
      return { type: "Point", coordinates: coordinates[0] };
    case "MultiPoint":
      return { type: "MultiPoint", coordinates };
    case "LineString":
      return { type: "LineString", coordinates };
    case "MultiLineString":
      return { type: "MultiLineString", coordinates: geometry.coordinates.map((c, i) => (i === path[0] ? coordinates : c)) };
    case "Polygon":
      return { type: "Polygon", coordinates: geometry.coordinates.map((c, i) => (i === path[0] ? coordinates : c)) };
    case "MultiPolygon":
      return {
        type: "MultiPolygon",
        coordinates: geometry.coordinates.map((rings, i) =>
          i === path[0] ? rings.map((c, j) => (j === path[1] ? coordinates : c)) : rings,
        ),
      };
    case "GeometryCollection":
      return geometry;
  }
}

function samePath(a: number[], b: number[]): boolean {
  return a.length === b.length && a.every((value, i) => value === b[i]);
}

function pathAt(geometry: GeoJSON.Geometry, ref: VertexRef): CoordinatePath | undefined {
  return readPaths(geometry).find((p) => samePath(p.path, ref.path));
}

export function moveVertex(geometry: GeoJSON.Geometry, ref: VertexRef, position: GeoJSON.Position): GeoJSON.Geometry {
  const target = pathAt(geometry, ref);
  if (!target) return geometry;
  const coordinates = target.coordinates.map((c, i) => (i === ref.index ? position : c));
  if (target.closed && ref.index === 0) coordinates[coordinates.length - 1] = position;
  return writePath(geometry, ref.path, coordinates);
}

// Inserts before `index` — the address a midpoint handle carries, so
// dragging it turns a segment into two.
export function insertVertex(geometry: GeoJSON.Geometry, ref: VertexRef, position: GeoJSON.Position): GeoJSON.Geometry {
  const target = pathAt(geometry, ref);
  if (!target) return geometry;
  const coordinates = [...target.coordinates];
  coordinates.splice(ref.index, 0, position);
  return writePath(geometry, ref.path, coordinates);
}

// Refuses the removal that would leave a shape that is no longer one: a
// ring under three distinct vertices, a line under two. Returns the
// geometry unchanged rather than an invalid one.
export function removeVertex(geometry: GeoJSON.Geometry, ref: VertexRef): GeoJSON.Geometry {
  const target = pathAt(geometry, ref);
  if (!target) return geometry;
  if (target.closed ? target.coordinates.length <= 4 : target.coordinates.length <= 2) return geometry;

  const coordinates = [...target.coordinates];
  coordinates.splice(ref.index, 1);
  if (target.closed && ref.index === 0) coordinates[coordinates.length - 1] = coordinates[0];
  return writePath(geometry, ref.path, coordinates);
}
