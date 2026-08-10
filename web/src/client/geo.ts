import { absoluteApiUrl, get } from './http'
import type { FilterCondition } from './types'

// filter, when non-empty, is JSON-encoded the same way GetItems/
// getItems takes it — the MVT tile endpoint applies it server-side
// (règle 5), so a client-controlled filter narrows what a tile renders
// exactly like it narrows a GeoJSON/OGC-Features query.
// version, when given, is appended as a plain query parameter the server
// ignores: the tile's content is already keyed on the table's UpdatedAt
// server-side, but a tile cache is keyed by URL, so a write has to change
// the URL for the map to ask again.
export function tileUrl(tableId: string, filter: FilterCondition[] = [], version = 0): string {
    const params = new URLSearchParams()
    if (filter.length > 0) params.set('filter', JSON.stringify(filter))
    if (version > 0) params.set('v', String(version))
    const qs = params.toString()
    return `${absoluteApiUrl(`/tables/${tableId}/tiles/{z}/{x}/{y}`)}${qs ? `?${qs}` : ''}`
}

// bbox is "minX,minY,maxX,maxY" in OGC:CRS84, same format as Metadata.bbox —
// empty when no row matches. Used to re-center the map on a filter's
// results instead of the whole table's extent.
export function getFilteredExtent(
    tableId: string,
    filter: FilterCondition[] = []
): Promise<{ bbox: string }> {
    const qs = filter.length > 0 ? `?filter=${encodeURIComponent(JSON.stringify(filter))}` : ''
    return get(`/tables/${tableId}/filtered-extent${qs}`)
}

// The distinct geometry types a table actually holds, computed server-side on
// every call — a geometry column may mix them, so it is not a stored fact.
// What it is read for: a table already homogeneous refuses a geometry of
// another family on write, so the editor stops offering to draw one.
export function getGeometryTypes(tableId: string): Promise<{ types: string[] }> {
    return get(`/tables/${tableId}/geometry-types`)
}
