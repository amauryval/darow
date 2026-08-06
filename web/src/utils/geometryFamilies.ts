export type GeometryFamily = 'point' | 'line' | 'polygon'

// The family a geometry type belongs to — the same reduction the server makes
// before refusing a write (`service.geometryFamily`): a table of polygons
// takes multipolygons, and nothing else. Anything outside the three families
// (a collection) constrains nothing.
export function geometryFamily(type: string): GeometryFamily | null {
    switch (type.trim().toUpperCase().replace(/^ST_/, '')) {
        case 'POINT':
        case 'MULTIPOINT':
            return 'point'
        case 'LINESTRING':
        case 'MULTILINESTRING':
            return 'line'
        case 'POLYGON':
        case 'MULTIPOLYGON':
            return 'polygon'
        default:
            return null
    }
}

// The single family a table is locked to, or null when it constrains nothing —
// an empty table (anything may be drawn first) or one already mixing families
// (the constraint is already broken and the server writes them all).
export function lockedFamily(types: string[]): GeometryFamily | null {
    const families = new Set<GeometryFamily>()
    for (const type of types) {
        const family = geometryFamily(type)
        if (family !== null) families.add(family)
    }
    return families.size === 1 ? [...families][0] : null
}

export const FAMILY_LABELS: Record<GeometryFamily, string> = {
    point: 'points',
    line: 'lignes',
    polygon: 'polygones'
}
