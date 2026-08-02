// Metadata.bbox and getFilteredExtent's bbox are both reported as
// "minX,minY,maxX,maxY" in OGC:CRS84 — this is the one place that parses
// that format, shared by anything that needs to hand a bbox to MapLibre's
// fitBounds or a vector source's bounds option.
export function parseBbox(bbox: string): [number, number, number, number] | null {
  const parts = bbox.split(",").map(Number);
  if (parts.length !== 4 || parts.some(Number.isNaN)) return null;
  return parts as [number, number, number, number];
}
