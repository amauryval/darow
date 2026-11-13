import { describe, expect, it } from 'vitest'
import { tableNameOf } from '@/darow/utils/tableName'

describe('tableNameOf', () => {
    it('drops the extension and lowers the case', () => {
        expect(tableNameOf('Communes.geojson')).toBe('communes')
    })

    it('replaces what a table name cannot hold', () => {
        expect(tableNameOf('Routes nationales-2024.gpkg')).toBe('routes_nationales_2024')
    })

    it('prefixes a name starting with a digit', () => {
        expect(tableNameOf('2024.csv')).toBe('_2024')
    })

    it('keeps at most 63 characters', () => {
        expect(tableNameOf(`${'a'.repeat(80)}.csv`)).toHaveLength(63)
    })
})
