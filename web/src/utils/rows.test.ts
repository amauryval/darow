import { describe, expect, it } from 'vitest'
import { cellValue, isHiddenColumn } from './rows'
import type { Row } from '../client/types'

const row: Row = { pkey: 'pk-1', name: 'Nantes', height: null }

describe('cellValue', () => {
    it('reads the primary key off Row.pkey rather than off the row body', () => {
        expect(cellValue(row, '__pkey')).toBe('pk-1')
    })

    it('reads any other column off the row itself', () => {
        expect(cellValue(row, 'name')).toBe('Nantes')
        expect(cellValue(row, 'height')).toBeNull()
    })

    it('yields undefined for a column the row does not carry', () => {
        expect(cellValue(row, 'absente')).toBeUndefined()
    })
})

describe('isHiddenColumn', () => {
    it('hides the server-managed columns', () => {
        expect(isHiddenColumn('__pkey')).toBe(true)
        expect(isHiddenColumn('OGC_FID')).toBe(true)
    })

    it('keeps a real data column', () => {
        expect(isHiddenColumn('geom')).toBe(false)
        expect(isHiddenColumn('ogc_fid')).toBe(false)
    })
})
