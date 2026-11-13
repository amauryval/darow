import { computed, ref } from 'vue'
import { sortList, type ListOrder } from '@myapplibs/web-platform/components/layout/listOrder'
import type { FilterInput } from '@/darow/api/darow'
import { useSavedFilters } from '@/darow/composables/useSavedFilters'
import { useTablesStore } from '@/darow/stores/tables'
import { distinctColumns } from '@/darow/utils/conditions'

export interface FilterDraft extends FilterInput {
    id: string
}

const search = ref('')
const order = ref<ListOrder>('label')
const draft = ref<FilterDraft | null>(null)

export function useFilterEditor() {
    const saved = useSavedFilters()
    const tables = useTablesStore()

    const items = computed(() => {
        const query = search.value.trim().toLowerCase()
        const found = saved.filters.value.filter(
            (filter) => filter.name.toLowerCase().includes(query) || filter.table.includes(query)
        )
        return sortList(
            found.map((filter) => ({ ...filter, label: filter.name })),
            order.value
        )
    })

    const open = (id: string) => {
        const filter = saved.get(id)
        draft.value = filter
            ? {
                  id: filter.id,
                  table: filter.table,
                  name: filter.name,
                  where: filter.where
              }
            : null
    }

    const create = () => {
        draft.value = { id: '', table: '', name: '', where: '' }
    }

    const close = () => {
        draft.value = null
    }

    const setTable = (table: string) => {
        if (draft.value) draft.value = { ...draft.value, table }
    }

    const columns = computed(() => {
        const table = draft.value?.table ?? ''
        return table ? tables.get(table)?.columns ?? [] : distinctColumns(tables.items)
    })

    const ready = computed(
        () => !!draft.value && draft.value.name.trim() !== '' && draft.value.where.trim() !== ''
    )

    const save = async (): Promise<boolean> => {
        if (!draft.value) return false
        const { id, ...input } = draft.value
        const item = await saved.save(id, input)
        if (item) open(item.id)
        return !!item
    }

    const remove = async (): Promise<boolean> => {
        if (!draft.value || !(await saved.remove(draft.value.id))) return false
        close()
        return true
    }

    return {
        search,
        order,
        items,
        draft,
        columns,
        ready,
        open,
        create,
        close,
        setTable,
        save,
        remove
    }
}
