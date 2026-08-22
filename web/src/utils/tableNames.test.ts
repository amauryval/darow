import { describe, expect, it } from 'vitest'
import { suggestTableName } from './tableNames'

describe('suggestTableName', () => {
    it('keeps the base name when it is free', () => {
        expect(suggestTableName('routes', ['autre'])).toBe('routes')
    })

    it('suffixes from 2 on the first collision', () => {
        expect(suggestTableName('routes', ['routes'])).toBe('routes (2)')
    })

    it('skips the suffixes already taken', () => {
        expect(suggestTableName('routes', ['routes', 'routes (2)', 'routes (3)'])).toBe(
            'routes (4)'
        )
    })
})
