// Metadata.bbox and getFilteredExtent's bbox are both reported as
// "minX,minY,maxX,maxY" in OGC:CRS84 — this is the one place that parses
// that format. What the map does with it goes through `projection.ts`,
// which is where lon/lat becomes a view extent.
export function parseBbox(bbox: string): [number, number, number, number] | null {
    const parts = bbox.split(',').map(Number)
    const [minX, minY, maxX, maxY] = parts
    if (parts.length !== 4 || parts.some(Number.isNaN)) return null
    return [minX, minY, maxX, maxY]
}
