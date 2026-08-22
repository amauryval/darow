import { describe, expect, it } from 'vitest'
import {
    SETTINGS_FIELD_GROUPS,
    TABLE_SETTINGS_FIELDS,
    inputRange,
    parseSettingsFieldErrors,
    tableFieldValue
} from './settingsFields'
import type { OverrideField, TableSettings } from '../client/types'

function overrideField(effective: number): OverrideField {
    return { unit: 'lignes', override: null, effective, minimum: 1 }
}

const settings: TableSettings = {
    tableId: 't1',
    isGeo: true,
    defaultRowLimit: overrideField(100),
    defaultFeaturesLimit: overrideField(5000),
    updatedAt: '2026-01-01T00:00:00Z'
}

describe('SETTINGS_FIELD_GROUPS', () => {
    it('gives every field a label and a plain-language description', () => {
        const fields = SETTINGS_FIELD_GROUPS.flatMap((group) => group.fields)

        expect(fields.length).toBeGreaterThan(0)
        expect(fields.every((field) => field.label !== '' && field.description !== '')).toBe(true)
    })

    it('names each key only once across the groups', () => {
        const keys = SETTINGS_FIELD_GROUPS.flatMap((group) => group.fields.map((f) => f.key))

        expect(new Set(keys).size).toBe(keys.length)
    })
})

describe('TABLE_SETTINGS_FIELDS', () => {
    it('holds exactly the per-table fields, in the global screen’s order', () => {
        expect(TABLE_SETTINGS_FIELDS.map((field) => field.key)).toEqual([
            'defaultRowLimit',
            'defaultFeaturesLimit',
            'maxSimplifyTolerance',
            'maxPrecision',
            'tileCacheMaxAgeSec',
            'coordinateDecimals'
        ])
    })

    it('leaves out the two global-only fields', () => {
        const keys = TABLE_SETTINGS_FIELDS.map((field) => field.key)

        expect(keys).not.toContain('defaultListLimit')
        expect(keys).not.toContain('cacheTTLSec')
    })
})

describe('inputRange', () => {
    it('answers an open range when the server published no bounds', () => {
        expect(inputRange(undefined)).toEqual({})
    })

    it('passes an inclusive minimum through', () => {
        expect(inputRange({ unit: 'lignes', minimum: 50, maximum: 5000 })).toEqual({
            min: 50,
            max: 5000
        })
    })

    it('turns an exclusive minimum into the first value the step grid allows', () => {
        expect(inputRange({ unit: 'm', exclusiveMinimum: 0 }, 0.01)).toEqual({
            min: 0.01,
            max: undefined
        })
    })

    it('steps an exclusive minimum by one when the field states no step', () => {
        expect(inputRange({ unit: 'lignes', exclusiveMinimum: 0 })).toEqual({
            min: 1,
            max: undefined
        })
    })
})

describe('tableFieldValue', () => {
    it('reads a field the table carries', () => {
        expect(tableFieldValue(settings, 'defaultRowLimit')?.effective).toBe(100)
    })

    it('answers null for a global-only field', () => {
        expect(tableFieldValue(settings, 'defaultListLimit')).toBeNull()
        expect(tableFieldValue(settings, 'cacheTTLSec')).toBeNull()
    })

    it('answers null for a geometry field the table does not carry', () => {
        expect(tableFieldValue(settings, 'maxPrecision')).toBeNull()
    })
})

describe('parseSettingsFieldErrors', () => {
    it('attaches each violation to the field it names', () => {
        expect(
            parseSettingsFieldErrors(
                'defaultRowLimit must be >= 50; defaultListLimit must be <= 5000'
            )
        ).toEqual({
            defaultRowLimit: 'defaultRowLimit must be >= 50',
            defaultListLimit: 'defaultListLimit must be <= 5000'
        })
    })

    it('ignores a segment that names no known field', () => {
        expect(parseSettingsFieldErrors('settings are invalid')).toEqual({})
    })

    it('ignores an empty message', () => {
        expect(parseSettingsFieldErrors('')).toEqual({})
    })
})
