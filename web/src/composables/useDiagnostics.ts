import { onMounted, ref } from 'vue'
import { getDiagnostics } from '../client/ops'
import type { Diagnostics } from '../client/types'
import { errorMessage } from '../client/http'

// Server-health section's data source. Per-mount, not module-level like
// useSettings — nothing else on the page needs to share this fetch.
export function useDiagnostics() {
    const data = ref<Diagnostics | null>(null)
    const loading = ref(false)
    const loadedOnce = ref(false)
    const error = ref('')

    async function load() {
        loading.value = true
        error.value = ''
        try {
            data.value = await getDiagnostics()
            loadedOnce.value = true
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de charger les diagnostics du serveur')
        } finally {
            loading.value = false
        }
    }

    onMounted(load)

    return { data, loading, loadedOnce, error, retry: load }
}
