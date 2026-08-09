import { onMounted, ref } from 'vue'
import { listTables } from '../client/tables'
import type { Metadata } from '../client/types'
import { errorMessage } from '../client/http'
import { computeDashboardStats, type DashboardStats } from '../utils/dashboardStats'
import { useSettings } from './useSettings'

// Fallback page size when /settings hasn't resolved yet — matches the
// server's own default cap (internal/config's MaxListLimit), so the first
// page this fires never gets clamped down mid-loop.
const FALLBACK_PAGE_SIZE = 500

// Data-overview section's source: unlike every other list in the app, the
// dashboard needs the *whole* catalogue to sum/aggregate, not one displayed
// page — so this is the one place allowed to page a listing to completion
// instead of going through usePaginatedList's infinite scroll.
export function useDashboardStats() {
    const settings = useSettings()
    const stats = ref<DashboardStats | null>(null)
    const loading = ref(false)
    const loadedOnce = ref(false)
    const error = ref('')

    async function load() {
        loading.value = true
        error.value = ''
        try {
            const limit = settings.current.value?.defaultListLimit.value ?? FALLBACK_PAGE_SIZE
            const tables: Metadata[] = []
            let offset = 0
            let total = Infinity
            while (offset < total) {
                const page = await listTables(offset, limit)
                tables.push(...page.tables)
                total = page.total
                offset += page.tables.length
                // A page answering fewer rows than asked with more still
                // declared in total would loop forever on a server bug —
                // stop rather than hang the composable.
                if (page.tables.length === 0) break
            }
            stats.value = computeDashboardStats(tables)
            loadedOnce.value = true
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de charger les statistiques des tables')
        } finally {
            loading.value = false
        }
    }

    onMounted(load)

    return { stats, loading, loadedOnce, error, retry: load }
}
