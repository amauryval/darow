import { describe, expect, it } from 'vitest'
import { COLUMN_TYPES, listed, SOURCE_FORMATS } from '../catalog'

describe('catalog', () => {
    it('lists the keys of a record, in order', () => {
        expect(listed<'a' | 'b'>({ a: true, b: true })).toEqual(['a', 'b'])
    })

    it('lists the column types and the source formats the server accepts', () => {
        expect(COLUMN_TYPES).toEqual([
            'VARCHAR',
            'BIGINT',
            'DOUBLE',
            'BOOLEAN',
            'DATE',
            'TIMESTAMP'
        ])
        expect(SOURCE_FORMATS).toEqual([
            '.geojson',
            '.json',
            '.csv',
            '.gpkg',
            '.parquet',
            '.fgb',
            '.kml'
        ])
    })
})
