import { describe, expect, it } from 'vitest'
import { computeDashboardStats } from './dashboardStats'
import type { Metadata } from '../client/types'

function table(tableName: string, rowCount: number, isGeo = true, srid = 4326): Metadata {
    return {
        tableId: tableName,
        physicalName: tableName,
        tableName,
        isGeo,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        rowCount,
        srid
    }
}

describe('computeDashboardStats', () => {
    it('counts the tables, their rows and the geo/non-geo split', () => {
        const stats = computeDashboardStats([
            table('routes', 10),
            table('stats', 5, false, 0),
            table('communes', 7)
        ])

        expect(stats.tableCount).toBe(3)
        expect(stats.totalRows).toBe(22)
        expect(stats.geoTableCount).toBe(2)
        expect(stats.nonGeoTableCount).toBe(1)
    })

    it('distributes the SRIDs of the geo tables only, most frequent first', () => {
        const stats = computeDashboardStats([
            table('a', 1, true, 2154),
            table('b', 1),
            table('c', 1, true, 2154),
            table('d', 1, false, 0)
        ])

        expect(stats.sridDistribution).toEqual([
            { srid: 2154, count: 2 },
            { srid: 4326, count: 1 }
        ])
    })

    it('ranks the top tables by row count and caps the list at eight', () => {
        const tables = Array.from({ length: 12 }, (_, i) => table(`t${i}`, i))

        const stats = computeDashboardStats(tables)

        expect(stats.topTablesByRows).toHaveLength(8)
        expect(stats.topTablesByRows[0]).toEqual({ tableName: 't11', rowCount: 11 })
        expect(stats.topTablesByRows.at(-1)).toEqual({ tableName: 't4', rowCount: 4 })
    })

    it('leaves the catalogue it was given in its own order', () => {
        const tables = [table('a', 1), table('b', 9)]

        computeDashboardStats(tables)

        expect(tables.map((t) => t.tableName)).toEqual(['a', 'b'])
    })

    it('answers zeroed stats for an empty catalogue', () => {
        expect(computeDashboardStats([])).toEqual({
            tableCount: 0,
            totalRows: 0,
            geoTableCount: 0,
            nonGeoTableCount: 0,
            sridDistribution: [],
            topTablesByRows: []
        })
    })
})
