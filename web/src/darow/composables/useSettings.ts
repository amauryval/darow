import { ref, shallowRef } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import type { Effective, TableSettings, Values } from '@/darow/api/settings.gen'
import * as api from '@/darow/api/darow'

const global = shallowRef<Effective | null>(null)
const table = shallowRef<TableSettings | null>(null)
const busy = ref(false)

export function useSettings() {
    const { attempt } = useAttempt()

    const loadGlobal = () =>
        attempt(
            t('darow.settings.loadFailed'),
            async () => {
                global.value = await api.getSettings()
            },
            busy
        )

    const saveGlobal = (values: Values) =>
        attempt(
            t('darow.settings.saveFailed'),
            async () => {
                global.value = await api.putSettings(values)
                return t('darow.settings.saved')
            },
            busy
        )

    const loadTable = (name: string) =>
        attempt(
            t('darow.settings.loadFailed'),
            async () => {
                table.value = await api.getTableSettings(name)
            },
            busy
        )

    const saveTable = (name: string, values: Values) =>
        attempt(
            t('darow.settings.saveFailed'),
            async () => {
                table.value = await api.putTableSettings(name, values)
                return t('darow.settings.saved')
            },
            busy
        )

    return { global, table, busy, loadGlobal, saveGlobal, loadTable, saveTable }
}
