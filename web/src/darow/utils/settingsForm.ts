import type { Effective, Values } from '@/darow/api/settings.gen'

export const SETTING_FIELDS = [
    'rowsPageSize',
    'featuresPageSize',
    'coordinateDecimals',
    'tileCacheSeconds'
] as const

export type SettingField = (typeof SETTING_FIELDS)[number]

export type SettingsForm = Record<SettingField, string>

const textOf = (value: number | null | undefined): string =>
    value === null || value === undefined ? '' : String(value)

export const formOfValues = (values: Values): SettingsForm => ({
    rowsPageSize: textOf(values.rowsPageSize),
    featuresPageSize: textOf(values.featuresPageSize),
    coordinateDecimals: textOf(values.coordinateDecimals),
    tileCacheSeconds: textOf(values.tileCacheSeconds)
})

export const formOfEffective = (effective: Effective): SettingsForm => formOfValues(effective)

const numberOf = (text: string): number | undefined => {
    const value = Number.parseInt(text.trim(), 10)
    return Number.isNaN(value) ? undefined : value
}

export const valuesOfForm = (form: SettingsForm): Values => ({
    rowsPageSize: numberOf(form.rowsPageSize),
    featuresPageSize: numberOf(form.featuresPageSize),
    coordinateDecimals: numberOf(form.coordinateDecimals),
    tileCacheSeconds: numberOf(form.tileCacheSeconds)
})
