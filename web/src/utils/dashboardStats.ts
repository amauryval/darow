import type { Metadata } from '../client/types'

const TOP_TABLES_COUNT = 8

export interface SridCount {
    srid: number
    count: number
}

export interface TableRowCount {
    tableName: string
    rowCount: number
}

export interface DashboardStats {
    tableCount: number
    totalRows: number
    geoTableCount: number
    nonGeoTableCount: number
    sridDistribution: SridCount[]
    topTablesByRows: TableRowCount[]
}

// computeDashboardStats derives the dashboard's data-overview numbers from
// the full table catalogue — the server exposes a page count (`total`) but
// no aggregate beyond it, so every other figure here is summed client-side
// over every page (see useDashboardStats).
export function computeDashboardStats(tables: Metadata[]): DashboardStats {
    const geoTables = tables.filter((table) => table.isGeo)

    const sridCounts = new Map<number, number>()
    for (const table of geoTables) {
        sridCounts.set(table.srid, (sridCounts.get(table.srid) ?? 0) + 1)
    }

    return {
        tableCount: tables.length,
        totalRows: tables.reduce((sum, table) => sum + table.rowCount, 0),
        geoTableCount: geoTables.length,
        nonGeoTableCount: tables.length - geoTables.length,
        sridDistribution: [...sridCounts.entries()]
            .map(([srid, count]) => ({ srid, count }))
            .sort((a, b) => b.count - a.count),
        topTablesByRows: [...tables]
            .sort((a, b) => b.rowCount - a.rowCount)
            .slice(0, TOP_TABLES_COUNT)
            .map((table) => ({ tableName: table.tableName, rowCount: table.rowCount }))
    }
}
