import Circle from "ol/style/Circle";
import Fill from "ol/style/Fill";
import Stroke from "ol/style/Stroke";
import Style from "ol/style/Style";
import MultiPoint from "ol/geom/MultiPoint";
import SimpleGeometry from "ol/geom/SimpleGeometry";
import type { FeatureLike } from "ol/Feature";
import type Geometry from "ol/geom/Geometry";
import type RenderFeature from "ol/render/Feature";
import type { Coordinate } from "ol/coordinate";

// The colours an edit session is drawn in, and the three states it keeps
// apart on one screen.
//
// The session draws *above* the table's own rendering, which stays on, so
// its colours cannot be the rendering's — indigo is the MVT layer's, green
// the WFS layer's (`mapStyles.ts`). Cyan is neither. Amber is the colour a
// pending write carries everywhere in the application, and it is free here
// because selection is suspended for the length of a session. Violet marks a
// shape that belongs to no row yet: saving it is an insert, not an update,
// and the two fail differently — a new shape with no attributes is what a
// save refuses.
const RESTING = "#0891b2";
const RESTING_DARK = "#0e7490";
const CHANGED = "#f59e0b";
const CHANGED_DARK = "#b45309";
const CREATED = "#7c3aed";
const CREATED_DARK = "#5b21b6";

// The property names the edit layer tags its features with, kept next to the
// style that has to read them on every frame.
export const DIRTY_PROPERTY = "dirty";
export const PKEY_PROPERTY = "pkey";

// Every vertex of a geometry, rebuilt only when that geometry actually moved.
// A style function runs once per feature per frame, and the vertices of one
// dragged geometry are wanted on every frame of the drag — so the answer is
// cached against the geometry's revision rather than recomputed, and one
// stale MultiPoint per geometry is all it ever holds.
interface CachedVertices {
  revision: number;
  points: MultiPoint | undefined;
}
const vertexCache = new WeakMap<Geometry, CachedVertices>();

export function verticesOf(geometry: Geometry | RenderFeature | undefined): MultiPoint | undefined {
  if (!(geometry instanceof SimpleGeometry)) return undefined;
  const revision = geometry.getRevision();
  const cached = vertexCache.get(geometry);
  if (cached && cached.revision === revision) return cached.points;
  const flat = geometry.getFlatCoordinates();
  const stride = geometry.getStride();
  const points: Coordinate[] = [];
  for (let i = 0; i + 1 < flat.length; i += stride) points.push([flat[i], flat[i + 1]]);
  const built = new MultiPoint(points);
  vertexCache.set(geometry, { revision, points: built });
  return built;
}

// A touched geometry is drawn heavier as well as recoloured: §8.7 asks that
// information never rest on colour alone, and a stroke width survives a
// colour-blind reading where a hue does not.
function bodyStyle(color: string, dark: string, opacity: number, width: number): Style[] {
  return [
    new Style({
      fill: new Fill({ color: rgba(color, opacity) }),
      stroke: new Stroke({ color: dark, width }),
      image: new Circle({ radius: 4, fill: new Fill({ color: dark }) }),
    }),
  ];
}

// The dots that say a limit can be grabbed, drawn from whatever the feature's
// geometry is at that frame — so they follow it through a drag rather than
// marking where it used to be.
function vertexStyle(dark: string): Style {
  return new Style({
    geometry: (feature) => verticesOf(feature.getGeometry()),
    image: new Circle({
      radius: 4.5,
      fill: new Fill({ color: "#ffffff" }),
      stroke: new Stroke({ color: dark, width: 1.5 }),
    }),
  });
}

function rgba(hex: string, alpha: number): string {
  const value = Number.parseInt(hex.slice(1), 16);
  return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`;
}

// Built once and handed back as they are: a style function is called for
// every feature of every frame, so it must not allocate.
const RESTING_STYLE = bodyStyle(RESTING, RESTING_DARK, 0.12, 2);
const CHANGED_STYLE = bodyStyle(CHANGED, CHANGED_DARK, 0.35, 3.5);
const CREATED_STYLE = bodyStyle(CREATED, CREATED_DARK, 0.35, 3.5);
const RESTING_VERTICES = vertexStyle(RESTING_DARK);
const CHANGED_VERTICES = vertexStyle(CHANGED_DARK);
const CREATED_VERTICES = vertexStyle(CREATED_DARK);

// What the legend in the edit bar names, so the three states are readable
// without decoding the colours (§8.7). The order is the one the bar shows.
export const EDIT_LEGEND = [
  { tone: RESTING_DARK, label: "Modifiable" },
  { tone: CHANGED_DARK, label: "Modifiée" },
  { tone: CREATED_DARK, label: "Nouvelle" },
];

// A feature with no pkey belongs to no row yet, whether or not it has been
// dragged since it was drawn: it is new, and stays new until it is saved.
function created(feature: FeatureLike): boolean {
  return feature.get(PKEY_PROPERTY) === undefined;
}

export function editStyle(feature: FeatureLike): Style[] {
  if (created(feature)) return CREATED_STYLE;
  return feature.get(DIRTY_PROPERTY) ? CHANGED_STYLE : RESTING_STYLE;
}

// The vertices of the one geometry the pointer rests over, drawn in that
// geometry's own state so the dots never contradict the shape under them.
//
// This is a layer of its own, holding that single feature, and that is the
// point: vertices for the whole session's worth of geometry meant thousands
// of points rebuilt on every frame of every pan and every drag. Kept apart,
// a change of what is hovered repaints one geometry instead of a whole view.
export function focusStyle(feature: FeatureLike): Style {
  if (created(feature)) return CREATED_VERTICES;
  return feature.get(DIRTY_PROPERTY) ? CHANGED_VERTICES : RESTING_VERTICES;
}

// The handle under the pointer — the one that will move if the user drags
// now, drawn heavier than a resting vertex so the target is unambiguous.
export const handleStyle = new Style({
  image: new Circle({
    radius: 6,
    fill: new Fill({ color: "#ffffff" }),
    stroke: new Stroke({ color: RESTING_DARK, width: 3 }),
  }),
});
