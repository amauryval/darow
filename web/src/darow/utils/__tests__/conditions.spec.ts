import { describe, expect, it } from 'vitest'
import {
    andAll,
    attributeColumns,
    columnIdentifier,
    distinctColumns,
    filterFits,
    insertToken,
    selectionClause,
    sqlIdentifier
} from '@/darow/utils/conditions'

const columns = [
    { name: 'geom', type: 'GEOMETRY' },
    { name: 'name', type: 'VARCHAR' },
    { name: 'pop', type: 'BIGINT' }
]

describe('attributeColumns', () => {
    it('leaves the geometries out', () => {
        expect(attributeColumns(columns).map((c) => c.name)).toEqual(['name', 'pop'])
    })
})

describe('sqlIdentifier', () => {
    it('quotes a name, its quotes doubled', () => {
        expect(sqlIdentifier('a"b')).toBe('"a""b"')
    })
})

describe('columnIdentifier', () => {
    it('qualifies a column by its table', () => {
        expect(columnIdentifier('pla"ces', 'pop')).toBe('"pla""ces"."pop"')
    })
})

describe('insertToken', () => {
    it('starts a condition, then follows it after a space', () => {
        expect(insertToken(' ', '"a"')).toBe('"a" ')
        expect(insertToken('"a" = 1 AND  ', '"b"')).toBe('"a" = 1 AND "b" ')
    })
})

describe('andAll', () => {
    it('keeps one condition as it is', () => {
        expect(andAll(['', ' "a" = 1 '])).toBe('"a" = 1')
    })

    it('puts several in parentheses joined by AND', () => {
        expect(andAll(['"a" = 1 OR "b" = 2', '"c" = 3'])).toBe('("a" = 1 OR "b" = 2) AND ("c" = 3)')
    })

    it('is blank without conditions', () => {
        expect(andAll([' '])).toBe('')
    })
})

describe('selectionClause', () => {
    it('keeps the ids', () => {
        expect(selectionClause([1, 3])).toBe('"__id" IN (1, 3)')
    })
})

describe('filterFits', () => {
    it('offers the filters of the table and of any table', () => {
        expect(filterFits({ table: 'places' }, 'places')).toBe(true)
        expect(filterFits({ table: '' }, 'places')).toBe(true)
        expect(filterFits({ table: 'roads' }, 'places')).toBe(false)
    })
})

describe('distinctColumns', () => {
    it('keeps each name once, in order', () => {
        const tables = [{ columns }, { columns: [columns[2], { name: 'len', type: 'DOUBLE' }] }]

        expect(distinctColumns(tables).map((c) => c.name)).toEqual(['geom', 'name', 'pop', 'len'])
    })
})
