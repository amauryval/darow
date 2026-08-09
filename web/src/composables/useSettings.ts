import { ref } from 'vue'
import { getSettings, updateSettings } from '../client/settings'
import type { Settings, SettingsPatch } from '../client/types'
import { errorMessage } from '../client/http'
import { parseSettingsFieldErrors, type SettingsFieldKey } from '../utils/settingsFields'
import { useToast } from './useToast'

// Module-level: every caller shares the one loaded value, which is what lets
// a list composable read a live cap (effectiveMaxListLimit, and the page
// sizes beside it) without each
// one fetching /settings on its own — the same sharing useToast's queue
// relies on. A cap lowered from SettingsView has to be visible everywhere a
// request size depends on it, not just on the settings screen itself.
const current = ref<Settings | null>(null)
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

    return { current, loading, loadedOnce, error, retry, saving, fieldErrors, save }
}
