import { absoluteApiUrl, get } from './http'
import type { OgcFeatureCollection } from './features'
import type { FilterCondition } from './types'

// filter, when non-empty, is JSON-encoded the same way GetItems/
// getTileFeatures take it — the MVT tile endpoint applies it server-side
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

export interface TileFeatureQueryOptions {
    limit?: number
    offset?: number
    simplify?: number
    filter?: FilterCondition[]
}

// Same z/x/y grid as tileUrl's MVT tiles, but as a GeoJSON FeatureCollection —
// the tile envelope is computed server-side (ST_TileEnvelope), so a client
// chunking its GeoJSON loading by tile never has to derive a tile's bbox
// itself. limit/offset page a single tile's results, same contract as OGC
// API - Features' items endpoint; simplify is the same tolerance override
// the MVT tile endpoint gets implicitly from its own zoom-based formula.
function getTileFeatures(
    tableId: string,
    z: number,
    x: number,
    y: number,
    opts: TileFeatureQueryOptions = {}
): Promise<OgcFeatureCollection> {
    const params = new URLSearchParams()
    if (opts.limit !== undefined) params.set('limit', String(opts.limit))
    if (opts.offset !== undefined) params.set('offset', String(opts.offset))
    if (opts.simplify !== undefined) params.set('simplify', String(opts.simplify))
    if (opts.filter !== undefined && opts.filter.length > 0)
        params.set('filter', JSON.stringify(opts.filter))
    const qs = params.toString()
    return get(`/tables/${tableId}/tiles/${z}/${x}/${y}/features${qs ? `?${qs}` : ''}`)
}

// A tile can hold more features than one page, and dropping the rest
// silently would make the map look like it is missing data at low zoom,
// where tiles are large — so a full page is always followed by the next one.
// The default matches the server's own defaultFeaturesLimit; this module
// stays Vue-independent (§2), so a caller that can read the live, operator-set
// value (useWfsLayer, through useSettings) passes it in as pageSize instead.
const DEFAULT_TILE_FEATURE_PAGE_SIZE = 10_000

// Every feature of one tile, paged. `complete` is false when a page request
// failed partway through: the caller gets what did load (rendering something
// beats rendering nothing) but must not present it as the tile's whole
// content (§8.6 — a silent partial result is a lie). A short page is not a
// failure, it is the normal end of the tile's data.
export async function getAllTileFeatures(
    tableId: string,
    z: number,
    x: number,
    y: number,
    opts: Omit<TileFeatureQueryOptions, 'limit' | 'offset'> & { pageSize?: number } = {}
): Promise<{ features: OgcFeatureCollection['features']; complete: boolean }> {
    const { pageSize = DEFAULT_TILE_FEATURE_PAGE_SIZE, ...queryOpts } = opts
    const features: OgcFeatureCollection['features'] = []
    for (let offset = 0; ; offset += pageSize) {
        let page
        try {
            page = await getTileFeatures(tableId, z, x, y, {
                ...queryOpts,
                limit: pageSize,
                offset
            })
        } catch {
            return { features, complete: false }
        }
        features.push(...page.features)
        if (page.features.length < pageSize) return { features, complete: true }
    }
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
