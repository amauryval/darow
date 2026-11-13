import { computed, ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { useTablesStore } from '@/darow/stores/tables'

const search = ref('')
const loading = ref(false)

export function useTables() {
    const store = useTablesStore()
    const { attempt } = useAttempt()

    const load = async () => {
        if (!store.loaded) await attempt(t('darow.tables.loadFailed'), store.fetch, loading)
    }

    const items = computed(() => {
        const query = search.value.trim().toLowerCase()
        return store.items.filter((table) => table.name.includes(query))
    })

    return { search, items, loading, load }
}
