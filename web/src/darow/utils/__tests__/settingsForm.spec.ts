import { describe, expect, it } from 'vitest'
import { formOfEffective, formOfValues, valuesOfForm } from '@/darow/utils/settingsForm'

describe('settingsForm', () => {
    it('writes set values as text, unset ones empty', () => {
        expect(formOfValues({ rowsPageSize: 20 })).toEqual({
            rowsPageSize: '20',
            featuresPageSize: '',
            coordinateDecimals: '',
            tileCacheSeconds: ''
        })
    })

    it('writes the effective settings', () => {
        expect(
            formOfEffective({
                rowsPageSize: 50,
                featuresPageSize: 10,
                coordinateDecimals: 7,
                tileCacheSeconds: 0
            }).tileCacheSeconds
        ).toBe('0')
    })

    it('reads the numbers back, an empty field left unset', () => {
        expect(
            valuesOfForm({
                rowsPageSize: ' 20 ',
                featuresPageSize: '',
                coordinateDecimals: 'x',
                tileCacheSeconds: '0'
            })
        ).toEqual({
            rowsPageSize: 20,
            featuresPageSize: undefined,
            coordinateDecimals: undefined,
            tileCacheSeconds: 0
        })
    })
})
