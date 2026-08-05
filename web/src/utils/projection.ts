import GeoJSONFormat from "ol/format/GeoJSON";
import { get as getProjection, transformExtent } from "ol/proj";
import { getIntersection, isEmpty, type Extent } from "ol/extent";
import type Feature from "ol/Feature";
import type Geometry from "ol/geom/Geometry";
import type Map from "ol/Map";
import { parseBbox } from "./bbox";

// The two coordinate systems the map lives between, and the only place they
// meet. Everything crossing `src/client/` is OGC:CRS84 lon/lat — the
// server's own contract, for a bbox as for a geometry read or written.
// Everything inside OpenLayers is Web Mercator, the projection the raster
// basemap is tiled in and the one the view measures distances in.
//
// Nothing else in the app converts between the two: a geometry that reached
// the REST client still in meters would be written into the table in
// meters, and nothing on screen would say so.
const DATA_PROJECTION = "EPSG:4326";
const VIEW_PROJECTION = "EPSG:3857";

// Configured once with both projections, so every read reprojects into the
// view and every write reprojects back out — the conversion is the format's
// job, never the caller's arithmetic.
const format = new GeoJSONFormat({ dataProjection: DATA_PROJECTION, featureProjection: VIEW_PROJECTION });

export function readGeometry(geometry: GeoJSON.Geometry): Geometry {
  return format.readGeometry(geometry);
}

export function writeGeometry(geometry: Geometry): GeoJSON.Geometry {
  return format.writeGeometryObject(geometry);
}

export function readFeatures(collection: GeoJSON.FeatureCollection): Feature<Geometry>[] {
  return format.readFeatures(collection);
}

// A "minX,minY,maxX,maxY" bbox as the view reads it, or null when the string
// is not one — an empty extent (a table with no row yet, a filter matching
// nothing) must leave the framing alone rather than zoom to a point.
export function viewExtent(bbox: string): Extent | null {
  const parsed = parseBbox(bbox);
  return parsed ? transformExtent(parsed, DATA_PROJECTION, VIEW_PROJECTION) : null;
}

// The other direction: what the map is looking at, as the lon/lat bbox every
// query in `src/client/` takes. Null when there is nothing to ask about yet.
//
// A view zoomed out past the world, or simply wider than it, looks at ground
// the projection cannot express — and the server transforms the bbox it is
// given back into Web Mercator, where a longitude of -250 has no image and
// the query fails outright. So the view is cut down to the projection's own
// extent first, which the engine knows and we do not have to restate.
//
// Null covers the two views there is no honest bbox for: one the engine has
// not measured (a container of zero size is reported as [0, 0] — an array,
// and truthy, but a view that is a point), and one that meets the world
// nowhere.
export function viewBbox(map: Map): [number, number, number, number] | null {
  const size = map.getSize();
  if (!size || size[0] <= 0 || size[1] <= 0) return null;

  const world = getProjection(VIEW_PROJECTION)?.getExtent();
  if (!world) return null;
  const visible = getIntersection(map.getView().calculateExtent(size), world);
  if (isEmpty(visible)) return null;

  const [minX, minY, maxX, maxY] = transformExtent(visible, VIEW_PROJECTION, DATA_PROJECTION);
  return [minX, minY, maxX, maxY];
}
