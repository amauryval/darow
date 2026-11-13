import * as api from '@/darow/api/darow'

export type TableLinkId = 'qgis' | 'geojson' | 'tiles' | 'csv'

export interface TableLink {
    id: TableLinkId
    url: string
    filtered: boolean
}

// tableLinks are the addresses another tool reads the table at, those of the data under where.
export const tableLinks = (name: string, hasGeometry: boolean, where: string): TableLink[] => {
    const filtered = where !== ''
    const csv: TableLink = { id: 'csv', url: api.exportUrl(name, 'csv', where, 0), filtered }
    if (!hasGeometry) return [csv]
    return [
        { id: 'qgis', url: api.ogcServiceUrl(), filtered: false },
        { id: 'geojson', url: api.exportUrl(name, 'geojson', where, 0), filtered },
        { id: 'tiles', url: api.tilesUrl(name, where), filtered },
        csv
    ]
}
