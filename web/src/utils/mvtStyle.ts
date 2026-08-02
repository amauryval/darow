import type { LayerSpecification, ExpressionSpecification } from "maplibre-gl";

// A geometry column can mix types — classify by substring so
// POINT/MULTIPOINT, LINESTRING/MULTILINESTRING and POLYGON/MULTIPOLYGON
// all match, and only the layer(s) matching what's actually present get
// added, instead of always drawing fill+line+circle regardless of type.
function classifyGeometryTypes(types: string[]) {
  return {
    hasPolygon: types.some((t) => t.includes("POLYGON")),
    hasLine: types.some((t) => t.includes("LINESTRING")),
    hasPoint: types.some((t) => t.includes("POINT")),
  };
}

// A feature-state boolean expression: highlighted styling for selected
// features, base styling otherwise.
function selectedCase(selectedValue: unknown, baseValue: unknown): ExpressionSpecification {
  return ["case", ["boolean", ["feature-state", "selected"], false], selectedValue, baseValue] as ExpressionSpecification;
}

export interface MvtLayerSpec {
  layer: LayerSpecification;
  // Only fill/circle layers register a dedicated hit target for click —
  // a polygon's "-line" outline stroke is a few px wide and would make
  // clicking its interior miss, so inspection always targets fill/circle
  // first.
  interactive: boolean;
}

// One vector layer per geometry type actually present in the table's
// geometry column, styled to highlight whichever features carry the
// "selected" feature-state.
export function buildMvtLayers(sourceId: string, sourceLayer: string, types: string[]): MvtLayerSpec[] {
  const { hasPolygon, hasLine, hasPoint } = classifyGeometryTypes(types);
  const specs: MvtLayerSpec[] = [];

  if (hasPolygon) {
    specs.push({
      interactive: true,
      layer: {
        id: `${sourceId}-fill`,
        type: "fill",
        source: sourceId,
        "source-layer": sourceLayer,
        paint: { "fill-color": selectedCase("#f59e0b", "#6366f1"), "fill-opacity": selectedCase(0.55, 0.35) },
      },
    });
  }
  if (hasPolygon || hasLine) {
    specs.push({
      interactive: false,
      layer: {
        id: `${sourceId}-line`,
        type: "line",
        source: sourceId,
        "source-layer": sourceLayer,
        paint: { "line-color": selectedCase("#b45309", "#4338ca"), "line-width": selectedCase(3, 1.5) },
      },
    });
  }
  if (hasPoint) {
    specs.push({
      interactive: true,
      layer: {
        id: `${sourceId}-point`,
        type: "circle",
        source: sourceId,
        "source-layer": sourceLayer,
        paint: {
          "circle-color": selectedCase("#f59e0b", "#4338ca"),
          "circle-radius": selectedCase(6, 4),
          "circle-stroke-color": "#ffffff",
          "circle-stroke-width": 1,
        },
      },
    });
  }
  return specs;
}
