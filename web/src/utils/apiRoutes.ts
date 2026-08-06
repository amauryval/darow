import { absoluteApiUrl } from '../client/http'

// One entry per read-only route a table exposes — what an external app
// copies to consume the table, never the CRUD/derive/saved-filter surface
// (spec/back/spec.md §6.1 has the full list; this is deliberately a subset).
export interface ApiRouteEntry {
    group: string
    method: 'GET'
    url: string
}

// {pkey}/{z}/{x}/{y}/{featureId} are left literal — the caller substitutes
// them, same convention as geo.ts's tileUrl.
export function buildTableApiRoutes(tableId: string, isGeo: boolean): ApiRouteEntry[] {
    const entries: ApiRouteEntry[] = [
        { group: 'Table', method: 'GET', url: absoluteApiUrl(`/tables/${tableId}`) },
        { group: 'Table', method: 'GET', url: absoluteApiUrl(`/tables/${tableId}/schema`) },
        {
            group: 'Lignes',
            method: 'GET',
            url: absoluteApiUrl(`/tables/${tableId}/rows?offset=0&limit=100&sort=`)
        },
        {
            group: 'Lignes',
            method: 'GET',
            url: absoluteApiUrl(`/tables/${tableId}/rows/{pkey}`)
        },
        {
            group: 'Export',
            method: 'GET',
            url: absoluteApiUrl(`/tables/${tableId}/export?format=${isGeo ? 'geojson' : 'csv'}`)
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
                group: 'Tuiles',
                method: 'GET',
                url: absoluteApiUrl(`/tables/${tableId}/tiles/{z}/{x}/{y}/features`)
            },
            {
                group: 'OGC API - Features',
                method: 'GET',
                url: absoluteApiUrl(`/ogc/collections/${tableId}`)
            },
            {
                group: 'OGC API - Features',
                method: 'GET',
                url: absoluteApiUrl(`/ogc/collections/${tableId}/items?bbox=&limit=100&offset=0`)
            },
            {
                group: 'OGC API - Features',
                method: 'GET',
                url: absoluteApiUrl(`/ogc/collections/${tableId}/items/{featureId}`)
            }
        )
    }

    return entries
}
