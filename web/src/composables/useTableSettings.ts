import { computed, ref, watch, type Ref } from 'vue'
import { deleteTableSettings, getTableSettings, updateTableSettings } from '../client/tablesettings'
import type { TableSettings, TableSettingsPatch } from '../client/types'
import { errorMessage } from '../client/http'
import { useSettings } from './useSettings'
import { useToast } from './useToast'

// A table's settings overrides: load, patch, reset, and its own four states
// (§0.18) — unlike useTableMetadata, a settings read can fail and the modal
// needs to show that rather than toast past it. Kept separate from
// useTableMetadata, whose subject is the table's own metadata, not its
// settings (§0.2). tableId-scoped like useTableMetadata, but not shared
// across callers — each caller gets its own load.
export function useTableSettings(tableId: Ref<string>) {
    const toast = useToast()
    const global = useSettings()
    const settings = ref<TableSettings | null>(null)
    const loading = ref(false)
    const loadedOnce = ref(false)
    const error = ref('')
    const saving = ref(false)

    async function load() {
        loading.value = true
        error.value = ''
        try {
            settings.value = await getTableSettings(tableId.value)
            loadedOnce.value = true
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de charger les paramètres de la table')
        } finally {
            loading.value = false
        }
    }

    function retry() {
        return load()
    }

    async function patch(body: TableSettingsPatch): Promise<boolean> {
        saving.value = true
        try {
            settings.value = await updateTableSettings(tableId.value, body)
            toast.success('Paramètres de la table enregistrés.')
            return true
        } catch (e) {
            toast.error(errorMessage(e, "Impossible d'enregistrer les paramètres de la table"))
            return false
        } finally {
            saving.value = false
        }
    }

    async function resetAll(): Promise<boolean> {
        saving.value = true
        try {
            settings.value = await deleteTableSettings(tableId.value)
            toast.success('Paramètres de la table réinitialisés.')
            return true
        } catch (e) {
            toast.error(errorMessage(e, 'Impossible de réinitialiser les paramètres de la table'))
            return false
        } finally {
            saving.value = false
        }
    }

    // The page size this table is actually served at: its own override when
    // it has one, the global row otherwise — the single place the two are
    // resolved, so no caller re-derives it from a constant.
    const rowLimit = computed(
        () => settings.value?.defaultRowLimit.effective ?? global.rowLimit.value
    )

    watch(tableId, load, { immediate: true })

    return { settings, rowLimit, loading, loadedOnce, error, retry, saving, patch, resetAll }
}
