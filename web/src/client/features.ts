import { get } from './http'
import type { FilterCondition } from './types'

interface OgcFeature {
    type: 'Feature'
    id: string
    geometry: GeoJSON.Geometry | null
    properties: Record<string, unknown>
}

interface OgcLink {
    rel: string
    type: string
    href: string
}

export interface OgcFeatureCollection {
    type: 'FeatureCollection'
    features: OgcFeature[]
    numberMatched: number
    numberReturned: number
    links: OgcLink[]
}

export interface FeatureQueryOptions {
    bbox?: [number, number, number, number]
    filter?: FilterCondition[]
    simplify?: number
    limit?: number
    offset?: number
}

function featureQueryParams(opts: FeatureQueryOptions): URLSearchParams {
    const params = new URLSearchParams()
    if (opts.bbox) params.set('bbox', opts.bbox.join(','))
    if (opts.filter?.length) params.set('filter', JSON.stringify(opts.filter))
    if (opts.simplify !== undefined) params.set('simplify', String(opts.simplify))
    if (opts.limit !== undefined) params.set('limit', String(opts.limit))
    if (opts.offset !== undefined) params.set('offset', String(opts.offset))
    return params
}

export function getItems(
    tableId: string,
    opts: FeatureQueryOptions = {},
    signal?: AbortSignal
): Promise<OgcFeatureCollection> {
    const qs = featureQueryParams(opts).toString()
    return get(`/ogc/collections/${tableId}/items${qs ? `?${qs}` : ''}`, signal)
}

// What is asked for, never what is served: the server resolves a requested
// limit against the operator's defaultFeaturesLimit (1000 out of the box) and
// answers against the one it chose, so the effective page is the smaller of
// the two.
const DEFAULT_ITEMS_PAGE_SIZE = 10_000

// Pages of the same collection are fetched several at a time, but a browser
// gives one host a handful of connections and every tile of the view pages at
// once — so the bound is per call and deliberately small, to leave room for
// the tiles that have not started yet.
const MAX_CONCURRENT_PAGES = 4

export interface ItemsPagingOptions extends Omit<FeatureQueryOptions, 'limit' | 'offset'> {
    pageSize?: number
    // Called once per page, with that page's features alone, as it arrives —
    // pages after the first come back out of order.
    onPage: (features: OgcFeatureCollection['features']) => void
}

// Every feature matching a query, paged. `complete` is false when a page
// failed: the caller keeps the pages that did arrive (rendering something
// beats rendering nothing) but must not present them as the whole answer
// (§8.6 — a silent partial result is a lie).
//
// The first page is what says how much there is: `numberMatched` is the total
// behind the query and the page's own length is the size the server actually
// served, so between the two the remaining offsets are known outright and are
// fetched in parallel instead of one round-trip after another.
export async function getAllItems(
    tableId: string,
    opts: ItemsPagingOptions
): Promise<{ complete: boolean }> {
    const { pageSize = DEFAULT_ITEMS_PAGE_SIZE, onPage, ...query } = opts

    let first
    try {
        first = await getItems(tableId, { ...query, limit: pageSize, offset: 0 })
    } catch {
        return { complete: false }
    }
    if (first.features.length > 0) onPage(first.features)

    const served = first.features.length
    if (served === 0 || served >= first.numberMatched) return { complete: true }

    const offsets: number[] = []
    for (let offset = served; offset < first.numberMatched; offset += served) offsets.push(offset)

    let complete = true
    let next = 0
    const worker = async () => {
        for (let i = next++; i < offsets.length; i = next++) {
            const offset = offsets[i]
            if (offset === undefined) return
            try {
                const page = await getItems(tableId, { ...query, limit: pageSize, offset })
                if (page.features.length > 0) onPage(page.features)
            } catch {
                complete = false
            }
        }
    }
    await Promise.all(
        Array.from({ length: Math.min(MAX_CONCURRENT_PAGES, offsets.length) }, worker)
    )
    return { complete }
}
