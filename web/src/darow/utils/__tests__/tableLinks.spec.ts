import { describe, expect, it } from 'vitest'
import { tableLinks } from '@/darow/utils/tableLinks'

const origin = window.location.origin
const table = `${origin}/api/v1/darow/tables/places`

describe('tableLinks', () => {
    it('offers the CSV alone for a table without geometry', () => {
        expect(tableLinks('places', false, '')).toEqual([
            { id: 'csv', url: `${table}/export?format=csv`, filtered: false }
        ])
    })

    it('offers the OGC service unfiltered and the data under the condition', () => {
        const where = encodeURIComponent('"pop" > 1').replace(/%20/g, '+')

        expect(tableLinks('places', true, '"pop" > 1')).toEqual([
            { id: 'qgis', url: `${origin}/api/v1/darow/ogc`, filtered: false },
            { id: 'geojson', url: `${table}/export?format=geojson&where=${where}`, filtered: true },
            { id: 'tiles', url: `${table}/tiles/{z}/{x}/{y}?where=${where}`, filtered: true },
            { id: 'csv', url: `${table}/export?format=csv&where=${where}`, filtered: true }
        ])
    })
})
