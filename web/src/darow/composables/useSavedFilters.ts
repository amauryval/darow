import { computed, ref, type Ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { useConfirm } from '@myapplibs/web-platform/components/composables/useConfirm'
import { ApiError } from '@myapplibs/web-platform/api/http'
import type { FilterInput } from '@/darow/api/darow'
import type { Filter } from '@/darow/api/filters.gen'
import { useFiltersStore } from '@/darow/stores/filters'
import { filterFits } from '@/darow/utils/conditions'

const loading = ref(false)

export function useSavedFilters() {
    const store = useFiltersStore()
    const { attempt } = useAttempt()
    const { confirmDialog } = useConfirm()

    const load = async () => {
        if (!store.loaded) await attempt(t('darow.filters.loadFailed'), store.fetch, loading)
    }

    const forTable = (table: Ref<string>) =>
        computed(() => store.items.filter((filter) => filterFits(filter, table.value)))

    const save = async (id: string, input: FilterInput): Promise<Filter | null> => {
        let saved: Filter | null = null
        await attempt(t('darow.filters.saveFailed'), async () => {
            const result = await store.save(id, {
                ...input,
                name: input.name.trim(),
                where: input.where.trim()
            })
            if (!result.ok) throw new ApiError(422, 'platform.invalid', {}, result.errors)
            saved = result.item
            return t('darow.filters.saved')
        })
        return saved
    }

    const remove = async (id: string): Promise<boolean> => {
        const filter = store.get(id)
        if (
            !filter ||
            !(await confirmDialog(t('darow.filters.confirmDelete', { name: filter.name })))
        )
            return false
        return attempt(t('darow.filters.deleteFailed'), async () => {
            await store.remove(id)
            return t('darow.filters.deleted')
        })
    }

    return {
        filters: computed(() => store.items),
        loading,
        load,
        get: store.get,
        forTable,
        save,
        remove
    }
}
