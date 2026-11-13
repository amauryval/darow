import type { ColumnType } from '@/darow/api/engine.gen'
import type { Extension } from '@/darow/api/sources.gen'

// listed gives the keys of a record over a union: one the server adds or drops fails the type-check.
export const listed = <T extends string>(all: Record<T, true>): T[] => Object.keys(all) as T[]

export const COLUMN_TYPES = listed<ColumnType>({
    VARCHAR: true,
    BIGINT: true,
    DOUBLE: true,
    BOOLEAN: true,
    DATE: true,
    TIMESTAMP: true
})

export const SOURCE_FORMATS = listed<Extension>({
    '.geojson': true,
    '.json': true,
    '.csv': true,
    '.gpkg': true,
    '.parquet': true,
    '.fgb': true,
    '.kml': true
})
