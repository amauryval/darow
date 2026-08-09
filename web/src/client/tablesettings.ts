import { del, get, patch } from './http'
import type { TableSettings, TableSettingsOverridesPage, TableSettingsPatch } from './types'

export function getTableSettings(tableId: string): Promise<TableSettings> {
    return get(`/tables/${tableId}/settings`)
}

export function updateTableSettings(
    tableId: string,
    body: TableSettingsPatch
): Promise<TableSettings> {
    return patch(`/tables/${tableId}/settings`, body)
}

export function deleteTableSettings(tableId: string): Promise<TableSettings> {
    return del(`/tables/${tableId}/settings`)
}

export function listOverriddenTableSettings(): Promise<TableSettings[]> {
    return get<TableSettingsOverridesPage>('/settings/tables').then((res) => res.tables)
}
