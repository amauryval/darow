import { absoluteApiUrl } from '../client/http'
import { LIST_PAGE_SIZE } from './pageSizes'

// One entry per read-only route a table exposes — what an external app
// copies to consume the table, never the CRUD/derive/saved-filter surface
// (spec/back/spec.md §6.1 has the full list; this is deliberately a subset).
export interface ApiRouteEntry {
    group: string
    method: 'GET'
    url: string
}

// {pkey}/{z}/{x}/{y}/{featureId} are left literal — the caller substitutes
// them, same convention as geo.ts's tileUrl. The listing URL carries a
// sample page size: what an outside client asks for is its own choice, the
// server resolving it against the operator's limit either way.
export function buildTableApiRoutes(tableId: string, isGeo: boolean): ApiRouteEntry[] {
    const entries: ApiRouteEntry[] = [
        { group: 'Table', method: 'GET', url: absoluteApiUrl(`/tables/${tableId}`) },
        { group: 'Table', method: 'GET', url: absoluteApiUrl(`/tables/${tableId}/schema`) },
        {
            group: 'Lignes',
            method: 'GET',
            url: absoluteApiUrl(`/tables/${tableId}/rows?offset=0&limit=${LIST_PAGE_SIZE}&sort=`)
        },
        {
            group: 'Lignes',
            method: 'GET',
            url: absoluteApiUrl(`/tables/${tableId}/rows/{pkey}`)
        }
    ]

    if (isGeo) {
        entries.push(
            {
                group: 'Lignes',
                method: 'GET',
                url: absoluteApiUrl(`/tables/${tableId}/rows/{pkey}/geometry`)
            },
            {
                group: 'Tuiles',
                method: 'GET',
                url: absoluteApiUrl(`/tables/${tableId}/tiles/{z}/{x}/{y}`)
            },
            {
                group: 'OGC API - Features',
                method: 'GET',
                url: absoluteApiUrl(`/ogc/collections/${tableId}`)
            }
        )
    }

    return entries
}
