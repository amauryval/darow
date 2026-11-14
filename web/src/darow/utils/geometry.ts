import type { FeatureCollection, Geometry } from 'geojson'
import { GeometryKey } from '@/darow/api/engine.gen'

const GEOMETRY_TYPES = [
    'Point',
    'MultiPoint',
    'LineString',
    'MultiLineString',
    'Polygon',
    'MultiPolygon',
    'GeometryCollection'
]

export const isGeometry = (value: unknown): value is Geometry =>
    typeof value === 'object' &&
    value !== null &&
    'type' in value &&
    typeof value.type === 'string' &&
    GEOMETRY_TYPES.includes(value.type)

export const geometryOf = (row: Record<string, unknown>): Geometry | null => {
    const value = row[GeometryKey]
    return isGeometry(value) ? value : null
}

export const geometryText = (geometry: Geometry | null): string =>
    geometry ? JSON.stringify(geometry, null, 2) : ''

export const parseGeometry = (text: string): Geometry | null | undefined => {
    if (text.trim() === '') return null
    try {
        const parsed: unknown = JSON.parse(text)
        return isGeometry(parsed) ? parsed : undefined
    } catch {
        return undefined
    }
}

export const sameGeometry = (a: Geometry | null, b: Geometry | null): boolean =>
    JSON.stringify(a) === JSON.stringify(b)

export const collectionOf = (geometry: Geometry | null): FeatureCollection => ({
    type: 'FeatureCollection',
    features: geometry ? [{ type: 'Feature', geometry, properties: {} }] : []
})
