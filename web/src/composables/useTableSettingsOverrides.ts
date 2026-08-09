import { ref } from 'vue'
import { deleteTableSettings, listOverriddenTableSettings } from '../client/tablesettings'
import type { TableSettings } from '../client/types'
import { errorMessage } from '../client/http'
import { useToast } from './useToast'

// The tables that carry a settings override, for the settings screen's own
// section — a dedicated route (GET /settings/tables), never the table
// catalogue filtered client-side: a deployment with a thousand tables must
// not load them all to find the few that override something.
export function useTableSettingsOverrides() {
    const toast = useToast()
    const tables = ref<TableSettings[]>([])
    const loading = ref(false)
    const loadedOnce = ref(false)
    const error = ref('')
    const resettingTableId = ref<string | null>(null)

    async function load() {
        loading.value = true
        error.value = ''
        try {
            tables.value = await listOverriddenTableSettings()
            loadedOnce.value = true
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de charger les dérogations par table')
        } finally {
            loading.value = false
        }
    }

    function retry() {
        return load()
    }

    async function resetTable(tableId: string) {
        resettingTableId.value = tableId
        try {
            await deleteTableSettings(tableId)
            tables.value = tables.value.filter((t) => t.tableId !== tableId)
            toast.success('Table réinitialisée aux valeurs globales.')
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de réinitialiser la table'))
        } finally {
            resettingTableId.value = null
        }
    }

    load()

    return { tables, loading, loadedOnce, error, retry, resettingTableId, resetTable }
}
