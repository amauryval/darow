import { computed, ref, shallowRef } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { useConfirm } from '@myapplibs/web-platform/components/composables/useConfirm'
import { pickIds, type PickMode } from '@myapplibs/web-platform/components/map/pickIds'
import type { Rows } from '@/darow/api/engine.gen'
import type { Filter } from '@/darow/api/filters.gen'
import type { RowRequest, TableDetail } from '@/darow/api/rest.gen'
import * as api from '@/darow/api/darow'
import { andAll, attributeColumns, selectionClause } from '@/darow/utils/conditions'
import { tableLinks } from '@/darow/utils/tableLinks'
import { NO_SORT, sameSort, type RowSort } from '@/darow/utils/rowValues'

const DEFAULT_PAGE_SIZE = 50

const name = ref('')
const detail = shallowRef<TableDetail | null>(null)
const rows = shallowRef<Rows>({ rows: [], total: 0 })
const page = ref(0)
const draft = ref('')
const applied = ref('')
const active = shallowRef<Filter | null>(null)
const selected = ref<number[]>([])
const selectionOnly = ref(false)
const conditions = computed(() =>
    andAll([
        applied.value,
        active.value?.where ?? '',
        selectionOnly.value && selected.value.length ? selectionClause(selected.value) : ''
    ])
)
const sort = shallowRef<RowSort>(NO_SORT)
const loading = ref(false)
const row = shallowRef<Record<string, unknown> | null>(null)
const pageSize = ref(DEFAULT_PAGE_SIZE)
const revision = ref(Date.now())

export function useTable() {
    const { attempt } = useAttempt()
    const { confirmDialog } = useConfirm()

    const fetchRows = async () => {
        rows.value = await api.listRows(name.value, {
            offset: page.value * pageSize.value,
            limit: pageSize.value,
            where: conditions.value,
            sort: sort.value
        })
    }

    const reload = () =>
        attempt(
            t('darow.table.loadFailed'),
            async () => {
                const [table, settings] = await Promise.all([
                    api.getTable(name.value),
                    api.getTableSettings(name.value)
                ])
                detail.value = table
                pageSize.value = settings.effective.rowsPageSize
                await fetchRows()
            },
            loading
        )

    const open = async (table: string) => {
        name.value = table
        detail.value = null
        page.value = 0
        draft.value = ''
        applied.value = ''
        active.value = null
        sort.value = NO_SORT
        row.value = null
        selected.value = []
        selectionOnly.value = false
        if (table) await reload()
    }

    const setPage = (next: number) => {
        page.value = next
        return attempt(t('darow.table.loadFailed'), fetchRows, loading)
    }

    const apply = (where: string = draft.value) => {
        draft.value = where.trim()
        applied.value = where.trim()
        return setPage(0)
    }

    const setSelected = (ids: number[]) => {
        selected.value = ids
        if (!selectionOnly.value) return
        if (!ids.length) selectionOnly.value = false
        return setPage(0)
    }

    const pick = (ids: number[], mode: PickMode) =>
        setSelected(pickIds(selected.value.map(String), ids.map(String), mode).map(Number))

    const toggleSelection = () => {
        selectionOnly.value = !selectionOnly.value && selected.value.length > 0
        return setPage(0)
    }

    const setSort = async (next: RowSort) => {
        if (sameSort(next, sort.value)) return
        sort.value = next
        await setPage(0)
    }

    const unapply = () => apply('')

    const toggleFilter = (filter: Filter) => {
        active.value = active.value?.id === filter.id ? null : filter
        return setPage(0)
    }

    const rename = async (to: string): Promise<boolean> =>
        attempt(t('darow.table.renameFailed'), async () => {
            await api.renameTable(name.value, to)
            name.value = to
            return t('darow.table.renamed')
        })

    const remove = async (): Promise<boolean> => {
        if (!(await confirmDialog(t('darow.table.confirmDelete', { name: name.value }))))
            return false
        return attempt(t('darow.table.deleteFailed'), async () => {
            await api.deleteTable(name.value)
            return t('darow.table.deleted')
        })
    }

    const openRow = (id: number) =>
        attempt(t('darow.row.loadFailed'), async () => {
            row.value = await api.getRow(name.value, id)
        })

    const saveRow = (id: number | null, request: RowRequest) =>
        attempt(t('darow.row.saveFailed'), async () => {
            row.value =
                id === null
                    ? await api.insertRow(name.value, request)
                    : await api.updateRow(name.value, id, request)
            revision.value++
            await reload()
            return t('darow.row.saved')
        })

    const saveCell = (id: number, column: string, text: string) =>
        attempt(t('darow.row.saveFailed'), async () => {
            await api.updateRow(name.value, id, { values: { [column]: text === '' ? null : text } })
            revision.value++
            await fetchRows()
        })

    const deleteRow = async (id: number): Promise<boolean> => {
        if (!(await confirmDialog(t('darow.row.confirmDelete')))) return false
        return attempt(t('darow.row.deleteFailed'), async () => {
            await api.deleteRow(name.value, id)
            row.value = null
            selected.value = selected.value.filter((other) => other !== id)
            if (!selected.value.length) selectionOnly.value = false
            revision.value++
            await reload()
            return t('darow.row.deleted')
        })
    }

    return {
        name,
        detail,
        rows,
        row,
        page,
        pageSize,
        draft,
        applied,
        active,
        activeId: computed(() => active.value?.id ?? ''),
        conditions,
        selected,
        selectionOnly,
        loading,
        attributes: computed(() => attributeColumns(detail.value?.columns ?? [])),
        hasGeometry: computed(() => !!detail.value?.geometry),
        pending: computed(() => draft.value.trim() !== applied.value),
        open,
        reload,
        setPage,
        setSort,
        apply,
        unapply,
        toggleFilter,
        pick,
        setSelected,
        clearSelection: () => setSelected([]),
        toggleSelection,
        rename,
        remove,
        openRow,
        selectRow: (selected: Record<string, unknown> | null) => (row.value = selected),
        newRow: () => (row.value = {}),
        saveRow,
        saveCell,
        changed: () => {
            revision.value++
            return reload()
        },
        deleteRow,
        download: (format: api.ExportFormat, srid: number) =>
            window.location.assign(api.exportUrl(name.value, format, conditions.value, srid)),
        tilesUrl: computed(() => api.tilesUrl(name.value, conditions.value, revision.value)),
        links: computed(() => tableLinks(name.value, !!detail.value?.geometry, conditions.value))
    }
}
