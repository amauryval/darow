import { describe, expect, it } from 'vitest'
import {
    NO_SORT,
    changedValues,
    draftOf,
    rowIdOf,
    rowSortOf,
    sameSort,
    textOf
} from '@/darow/utils/rowValues'

const columns = [
    { name: 'name', type: 'VARCHAR' },
    { name: 'pop', type: 'BIGINT' }
]

describe('textOf', () => {
    it('writes a value as text, nothing for null', () => {
        expect(textOf(12)).toBe('12')
        expect(textOf(true)).toBe('true')
        expect(textOf(null)).toBe('')
        expect(textOf({ a: 1 })).toBe('{"a":1}')
    })
})

describe('draftOf', () => {
    it('holds the text of each column', () => {
        expect(draftOf(columns, { __id: 1, name: 'Lyon', pop: null })).toEqual({
            name: 'Lyon',
            pop: ''
        })
    })
})

describe('changedValues', () => {
    const row = { __id: 1, name: 'Lyon', pop: 500 }

    it('keeps only what changed', () => {
        expect(changedValues(columns, row, { name: 'Lyon', pop: '600' })).toEqual({ pop: '600' })
    })

    it('empties a cleared value', () => {
        expect(changedValues(columns, row, { name: '', pop: '500' })).toEqual({ name: null })
    })

    it('is empty when nothing changed', () => {
        expect(changedValues(columns, row, draftOf(columns, row))).toEqual({})
    })
})

describe('rowIdOf', () => {
    it('reads the row number', () => {
        expect(rowIdOf({ __id: 3 })).toBe(3)
        expect(rowIdOf({})).toBeNull()
    })
})

describe('rowSortOf', () => {
    it('reads the first sort of the table, a column without its prefix', () => {
        expect(rowSortOf([])).toEqual(NO_SORT)
        expect(rowSortOf([{ id: 'col-name', desc: true }])).toEqual({ column: 'name', desc: true })
        expect(rowSortOf([{ id: '__id', desc: false }])).toEqual({ column: '__id', desc: false })
    })
})

describe('sameSort', () => {
    it('compares the column and the direction', () => {
        expect(sameSort({ column: 'a', desc: true }, { column: 'a', desc: true })).toBe(true)
        expect(sameSort({ column: 'a', desc: true }, { column: 'a', desc: false })).toBe(false)
        expect(sameSort(NO_SORT, { column: 'a', desc: false })).toBe(false)
    })
})
