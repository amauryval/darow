import { absoluteApiUrl, get, post } from './http'
import type { FilterCondition } from './types'
import type { OgcFeatureCollection } from './features'

// filter, when non-empty, is JSON-encoded like getItems takes it and applied
// server-side (rule 5). version is a parameter the server ignores: the tile
// is already keyed on UpdatedAt server-side, but a browser tile cache is
// keyed by URL, so a write has to change the URL for the map to ask again.
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

// The distinct geometry types a table holds, computed server-side on every
// call — a column may mix them, so it is not a stored fact. A homogeneous
// table refuses another family on write, so the editor stops offering it.
export function getGeometryTypes(tableId: string): Promise<{ types: string[] }> {
    return get(`/tables/${tableId}/geometry-types`)
}

// The rows whose geometry lies within buffer meters of one geometry, at full
// resolution and in OGC:CRS84 — what an edit session pulls in around the row
// the pointer reached, so a shared limit has both its sides on the map.
// exclude drops the rows the caller already holds.
export function getNeighbours(
    tableId: string,
    geometry: GeoJSON.Geometry,
    buffer: number,
    exclude: string[],
    limit: number
): Promise<OgcFeatureCollection> {
    return post(`/tables/${tableId}/neighbours`, { geometry, buffer, exclude, limit })
}
