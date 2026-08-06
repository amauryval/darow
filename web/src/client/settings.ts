import { get, patch } from './http'
import type { Settings, SettingsPatch } from './types'

export function getSettings(): Promise<Settings> {
    return get('/settings')
}

// A rejected patch (400) leaves every setting unchanged server-side; the
// caller still holds its own draft to retry from.
export function updateSettings(body: SettingsPatch): Promise<Settings> {
    return patch('/settings', body)
}
