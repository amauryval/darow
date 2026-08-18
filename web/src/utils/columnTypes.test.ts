import { describe, expect, it } from 'vitest'
import { coerceByType, columnKind, columnType, isTextType, typeError } from './columnTypes'

describe('columnType', () => {
    const columns = [
        { name: 'name', type: 'VARCHAR' },
        { name: 'height', type: 'DOUBLE' }
    ]

    it('reads the declared type of a known column', () => {
        expect(columnType(columns, 'height')).toBe('DOUBLE')
    })

    it('answers the empty string for an unknown column', () => {
        expect(columnType(columns, 'absente')).toBe('')
    })
})

describe('isTextType', () => {
    it('holds for the text-ish types worth suggesting values for', () => {
        expect(isTextType('VARCHAR')).toBe(true)
        expect(isTextType('text')).toBe(true)
        expect(isTextType('UUID')).toBe(true)
    })

    it('fails for numbers, dates and booleans', () => {
        expect(isTextType('DOUBLE')).toBe(false)
        expect(isTextType('TIMESTAMP')).toBe(false)
        expect(isTextType('BOOLEAN')).toBe(false)
    })
})

describe('columnKind', () => {
    it('reduces the database type names to the four form behaviours', () => {
        expect(columnKind('BOOLEAN')).toBe('boolean')
        expect(columnKind('TIMESTAMP WITH TIME ZONE')).toBe('timestamp')
        expect(columnKind('DATETIME')).toBe('timestamp')
        expect(columnKind('DATE')).toBe('date')
        expect(columnKind('BIGINT')).toBe('integer')
        expect(columnKind('HUGEINT')).toBe('integer')
        expect(columnKind('DECIMAL(10,2)')).toBe('number')
        expect(columnKind('VARCHAR')).toBe('text')
    })

    it('reads a timestamp before the date it contains', () => {
        expect(columnKind('timestamp')).toBe('timestamp')
    })
})

describe('coerceByType', () => {
    it('writes NULL for the empty string, whatever the kind', () => {
        expect(coerceByType('BOOLEAN', '')).toBeNull()
        expect(coerceByType('INTEGER', '')).toBeNull()
        expect(coerceByType('VARCHAR', '')).toBeNull()
    })

    it('reads a boolean from "true" or "1", anything else being false', () => {
        expect(coerceByType('BOOLEAN', 'true')).toBe(true)
        expect(coerceByType('BOOLEAN', '1')).toBe(true)
        expect(coerceByType('BOOLEAN', 'false')).toBe(false)
    })

    it('reads a number, keeping the raw text when it is not one', () => {
        expect(coerceByType('DOUBLE', '1.5')).toBe(1.5)
        expect(coerceByType('INTEGER', '-3')).toBe(-3)
        expect(coerceByType('INTEGER', 'trois')).toBe('trois')
    })

    it('passes any other kind through as text', () => {
        expect(coerceByType('DATE', '2026-01-01')).toBe('2026-01-01')
    })
})

describe('typeError', () => {
    it('accepts the empty string on every kind — it is a NULL, not a violation', () => {
        expect(typeError('INTEGER', '')).toBeNull()
        expect(typeError('DATE', '')).toBeNull()
    })

    it('states what a boolean expects', () => {
        expect(typeError('BOOLEAN', 'true')).toBeNull()
        expect(typeError('BOOLEAN', 'oui')).toBe('Valeur attendue : vrai ou faux')
    })

    it('refuses a fractional value on an integer column', () => {
        expect(typeError('INTEGER', '3')).toBeNull()
        expect(typeError('INTEGER', '3.5')).toBe('Nombre entier attendu')
    })

    it('refuses text on a number column', () => {
        expect(typeError('DOUBLE', '3.5')).toBeNull()
        expect(typeError('DOUBLE', 'trois')).toBe('Nombre attendu')
    })

    it('states the expected date format', () => {
        expect(typeError('DATE', '2026-01-01')).toBeNull()
        expect(typeError('DATE', '01/01/2026')).toBe('Date attendue, au format AAAA-MM-JJ')
    })

    it('states the expected timestamp format', () => {
        expect(typeError('TIMESTAMP', '2026-01-01 10:30')).toBeNull()
        expect(typeError('TIMESTAMP', 'hier')).toBe(
            'Date et heure attendues, au format AAAA-MM-JJ hh:mm'
        )
    })

    it('accepts anything on a text column', () => {
        expect(typeError('VARCHAR', 'n’importe quoi')).toBeNull()
    })
})
