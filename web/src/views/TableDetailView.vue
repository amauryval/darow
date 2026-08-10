<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { SortingState } from '@tanstack/vue-table'
import { exportTable } from '../client/tables'
import type { ExportFormat } from '../client/types'
import { errorMessage } from '../client/http'
import ColumnsModal from '../components/tables/ColumnsModal.vue'
import TableDataPanel from '../components/tables/TableDataPanel.vue'
import TableHeader from '../components/tables/TableHeader.vue'
import DeriveTableModal from '../components/tables/DeriveTableModal.vue'
import AggregateTableModal from '../components/tables/AggregateTableModal.vue'
import AddColumnModal from '../components/tables/AddColumnModal.vue'
import ApiRoutesModal from '../components/tables/ApiRoutesModal.vue'
import ExportModal from '../components/tables/ExportModal.vue'
import TableSettingsModal from '../components/tables/TableSettingsModal.vue'
import Breadcrumbs from '../components/ui/Breadcrumbs.vue'
import ErrorState from '../components/ui/ErrorState.vue'
import Skeleton from '../components/ui/Skeleton.vue'
import { useFeatureSelection } from '../composables/useFeatureSelection'
import { useTableMetadata } from '../composables/useTableMetadata'
import { useToast } from '../composables/useToast'
import { useDocumentTitle } from '../composables/useDocumentTitle'
import { queryString } from '../utils/routeQuery'
import { parseSorting, stringifySorting } from '../utils/sort'

const props = defineProps<{ tableId: string }>()
const router = useRouter()
const route = useRoute()
const toast = useToast()

// The applied filter and the sort are the workspace's own URL state (§9): read
// once here — TableDataPanel/TableFilterBar own the rest of the sync, this
// view only mirrors their current value into the query string. The filter
// identifier belongs to one table and is dropped by TableFilterBar's own
// tableId watch on navigation; an id the URL names that the catalogue no
// longer holds simply applies no filter (TableFilterBar.activeFilter), never
// an error.
const activeFilterId = ref<string | null>(queryString(route.query.filterId) || null)
const initialSort = queryString(route.query.sort)
    ? parseSorting(queryString(route.query.sort))
    : undefined
const sort = ref<SortingState>(initialSort ?? [])

watch([activeFilterId, sort], ([filterId, sorting]) => {
    router.replace({
        query: {
            ...route.query,
            filterId: filterId ?? undefined,
            sort: stringifySorting(sorting) || undefined
        }
    })
})

const {
    table,
    columns,
    rowColumns,
    loading,
    loadedOnce,
    error,
    retry,
    loadTable,
    rename,
    remove,
    addNewColumn
} = useTableMetadata(toRef(props, 'tableId'))

useDocumentTitle(computed(() => table.value?.tableName ?? 'Table'))

// Selection is owned here rather than in the data panel: what is selected is
// a count of the table's rows, and it is stated where the table's other
// counters are (TableHeader). The panel and the map act on it through this
// one instance — there is a single selection, map and grid alike (§8.8).
const selection = useFeatureSelection(toRef(props, 'tableId'))

const renaming = ref(false)
const editingName = ref(false)

// The user's visible-column choice must survive a schema reload (§8.3): only
// the first population is a straight assignment, later ones diff against the
// previously known column list so a hidden column stays hidden, a removed
// column drops out, and a newly added one defaults to visible.
const visibleColumns = ref<string[]>([])
let knownRowColumns: string[] = []
// Another table's columns are not a schema reload: forgetting what was known
// makes the next population a straight assignment again, so the new table
// opens on all of its columns rather than on the previous one's choice (§9).
watch(toRef(props, 'tableId'), () => {
    knownRowColumns = []
    editingName.value = false
})
watch(
    rowColumns,
    (cols) => {
        if (knownRowColumns.length === 0) {
            visibleColumns.value = cols
        } else {
            const added = cols.filter((c) => !knownRowColumns.includes(c))
            visibleColumns.value = [
                ...visibleColumns.value.filter((c) => cols.includes(c)),
                ...added
            ]
        }
        knownRowColumns = cols
    },
    { immediate: true }
)

const showColumns = ref(false)
const showDerive = ref(false)
const showAggregate = ref(false)
const showAddColumn = ref(false)
const showApiRoutes = ref(false)
const showTableSettings = ref(false)

const addColumnModalRef = ref<InstanceType<typeof AddColumnModal>>()

// The rename form closes on success only: a rejected name has to stay in
// front of the user, in the field they typed it in (§8.4).
async function onRename(name: string) {
    renaming.value = true
    try {
        await rename(name)
        editingName.value = false
    } catch (e) {
        toast.error(errorMessage(e, 'Impossible de renommer la table'))
    } finally {
        renaming.value = false
    }
}

async function onDeleteTable() {
    try {
        const deleted = await remove()
        toast.success(`Table "${deleted.name}" supprimée.`)
        router.push('/tables')
    } catch (e) {
        toast.error(errorMessage(e, 'Impossible de supprimer la table'))
    }
}

const exporting = ref(false)
const showExport = ref(false)

// The format and the output CRS are the modal's to choose (§5.6): this only
// runs the download and reports a failure the way every other action does.
async function onExport(format: ExportFormat, srid: number | null) {
    if (!table.value) return
    exporting.value = true
    try {
        await exportTable(table.value.tableId, format, srid, table.value.tableName)
    } catch (e) {
        toast.error(errorMessage(e, "Impossible d'exporter la table"))
    } finally {
        exporting.value = false
    }
}

async function onDeriveSuccess(result: { tableId: string }) {
    await router.push(`/tables/${result.tableId}`)
}

function openAddColumn() {
    showAddColumn.value = true
    addColumnModalRef.value?.openModal()
}

async function onColumnAdded(name: string, type: string) {
    await addNewColumn(name, type)
    await loadTable()
}
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-3 p-6">
        <Breadcrumbs
            :items="[{ label: 'Tables', to: '/tables' }, { label: table?.tableName ?? '…' }]"
        />

        <Skeleton v-if="loading && !loadedOnce" :rows="6" />
        <ErrorState v-else-if="error" :message="error" @retry="retry" />

        <TableHeader
            v-if="table"
            :table="table"
            :editing-name="editingName"
            :selected-count="selection.selectedPkeys.value.length"
            :zooming="selection.zooming.value"
            :renaming="renaming"
            :exporting="exporting"
            @update:editing-name="editingName = $event"
            @clear-selection="selection.clear"
            @zoom-to-selection="selection.zoomToSelection"
            @rename="onRename"
            @add-column="openAddColumn"
            @derive="showDerive = true"
            @aggregate="showAggregate = true"
            @export="showExport = true"
            @api-routes="showApiRoutes = true"
            @table-settings="showTableSettings = true"
            @delete="onDeleteTable"
        />

        <TableDataPanel
            v-if="table"
            :table-id="table.tableId"
            :bbox="table.bbox"
            :is-geo="table.isGeo"
            :row-columns="rowColumns"
            :visible-columns="visibleColumns"
            :columns="columns"
            :geometry-column="table.geometryColumn"
            :selection="selection"
            :active-filter-id="activeFilterId"
            :initial-sort="initialSort"
            @mutated="loadTable"
            @open-columns="showColumns = true"
            @update:active-filter-id="activeFilterId = $event"
            @update:sort="sort = $event"
        />

        <DeriveTableModal
            v-if="showDerive && table"
            :table-id="table.tableId"
            :columns="columns"
            :geometry-column="table.geometryColumn"
            @derived="onDeriveSuccess"
            @close="showDerive = false"
        />

        <AggregateTableModal
            v-if="showAggregate && table"
            :table-id="table.tableId"
            :columns="columns"
            :geometry-column="table.geometryColumn"
            @derived="onDeriveSuccess"
            @close="showAggregate = false"
        />

        <ColumnsModal
            v-if="showColumns && table"
            :table-id="table.tableId"
            :row-columns="rowColumns"
            :visible-columns="visibleColumns"
            @update:visible-columns="visibleColumns = $event"
            @close="showColumns = false"
        />

        <AddColumnModal
            v-if="showAddColumn && table"
            ref="addColumnModalRef"
            :on-add="onColumnAdded"
            @close="showAddColumn = false"
        />

        <ExportModal
            v-if="showExport && table"
            :table="table"
            :exporting="exporting"
            :on-export="onExport"
            @close="showExport = false"
        />

        <ApiRoutesModal
            v-if="showApiRoutes && table"
            :table-id="table.tableId"
            :is-geo="table.isGeo"
            @close="showApiRoutes = false"
        />

        <TableSettingsModal
            v-if="showTableSettings && table"
            :table-id="table.tableId"
            :is-geo="table.isGeo"
            @close="showTableSettings = false"
        />
    </div>
</template>
