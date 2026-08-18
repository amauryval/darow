import { computed, ref } from 'vue'
import { getSettings, updateSettings } from '../client/settings'
import type { Settings, SettingsPatch } from '../client/types'
import { errorMessage } from '../client/http'
import { parseSettingsFieldErrors, type SettingsFieldKey } from '../utils/settingsFields'
import { useToast } from './useToast'

// Module-level: every caller shares the one loaded value, which is what lets
// a list composable read a live page size without each one fetching
// /settings on its own — the same sharing useToast's queue relies on. A cap
// lowered from SettingsView has to be visible everywhere a request size
// depends on it, not just on the settings screen itself.
const current = ref<Settings | null>(null)

// The server's own seed values (internal/settings' constants), standing in
// only for the window before /settings has answered.
const SEED_ROW_LIMIT = 100
const SEED_LIST_LIMIT = 50

// The two operator-set page sizes the application deliberately follows rather
// than choosing for itself: the row limit a table is served at (through
// useTableSettings, which prefers the table's own override) and the list limit
// the dashboard's full sweep pages at, so a sweep never asks for more than one
// response can carry. Every other list pages on this client's own choice
// (utils/pageSizes).
const rowLimit = computed(() => current.value?.defaultRowLimit.value ?? SEED_ROW_LIMIT)
const listLimit = computed(() => current.value?.defaultListLimit.value ?? SEED_LIST_LIMIT)
const loading = ref(false)
const loadedOnce = ref(false)
const error = ref('')
let inFlight: Promise<void> | null = null

async function load() {
    loading.value = true
    error.value = ''
    try {
        current.value = await getSettings()
        loadedOnce.value = true
    } catch (e) {
        error.value = errorMessage(e, 'Impossible de charger les paramètres')
    } finally {
        loading.value = false
    }
}

function ensureLoaded(): Promise<void> {
    if (!inFlight) inFlight = load()
    return inFlight
}

// The settings row, its four-state load, a draft to edit and save. Every
// caller triggers the same shared load — a second mount while the first is
// still loading joins the same request rather than firing a second one.
export function useSettings() {
    ensureLoaded()
    const toast = useToast()

    const saving = ref(false)
    const fieldErrors = ref<Partial<Record<SettingsFieldKey, string>>>({})

    function retry() {
        inFlight = null
        return ensureLoaded()
    }

    // Saves patch (only the fields the form actually changed) and reports the
    // result as a global action's result (§8.4): a toast either way, and the
    // rejection additionally broken down per field for the form to attach.
    async function save(patch: SettingsPatch): Promise<boolean> {
        saving.value = true
        fieldErrors.value = {}
        try {
            current.value = await updateSettings(patch)
            toast.success('Paramètres enregistrés.')
            return true
        } catch (e) {
            const message = errorMessage(e, "Impossible d'enregistrer les paramètres")
            fieldErrors.value = parseSettingsFieldErrors(message)
            toast.error(message)
            return false
        } finally {
            saving.value = false
        }
    }

    return {
        current,
        rowLimit,
        listLimit,
        loading,
        loadedOnce,
        error,
        retry,
        saving,
        fieldErrors,
        save
    }
}
