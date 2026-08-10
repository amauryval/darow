<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue'
import type { SortingState } from '@tanstack/vue-table'
import type { ColumnSchema, Row } from '../../client/types'
import type { LayerChanges } from '../../composables/useEditLayer'
import type { useFeatureSelection } from '../../composables/useFeatureSelection'
import { useLayerEditing } from '../../composables/useLayerEditing'
import { isDraft, useRowEdits } from '../../composables/useRowEdits'
import { useRowFilter } from '../../composables/useRowFilter'
import { useToast } from '../../composables/useToast'
import { useRowGeometry } from '../../composables/useRowGeometry'
import { useTableSettings } from '../../composables/useTableSettings'
import { PAGE_SIZE, useTableRows } from '../../composables/useTableRows'
import FilteredRowsPanel from '../filters/FilteredRowsPanel.vue'
import GeometryModal from './GeometryModal.vue'
import TableEditBar from './TableEditBar.vue'
import TableMapPanel from './TableMapPanel.vue'
import TableRowsPanel from './TableRowsPanel.vue'
import TableToolbar from './TableToolbar.vue'
import TransformColumnModal from './TransformColumnModal.vue'

const props = defineProps<{
    tableId: string
    bbox?: string
    isGeo: boolean
    rowColumns: string[]
    visibleColumns: string[]
    columns: ColumnSchema[]
    geometryColumn?: string
    // Owned by the view: what is selected is one of the table's counters, and
    // it is stated with the others (TableHeader). One instance, so the map and
    // the grid share the single selection they are supposed to (§8.8).
    selection: ReturnType<typeof useFeatureSelection>
    // The applied filter and the grid's sort are the view's state, carried in
    // the URL (§9) — read once at creation (initialSort) or controlled
    // (activeFilterId), the two traps of TODO-F33.
    activeFilterId: string | null
    initialSort?: SortingState
}>()
// mutated: after any row add/edit/delete, since that changes the table's
// rowCount — the parent owns table metadata and reloads it.
// openColumns: the visible-column choice is the view's state (it survives a
// schema reload), so the toolbar's button only asks for its modal.
const emit = defineEmits<{
    mutated: []
    openColumns: []
    'update:activeFilterId': [id: string | null]
    'update:sort': [sort: SortingState]
}>()

const tableId = toRef(props, 'tableId')
const columns = toRef(props, 'columns')
const isGeo = toRef(props, 'isGeo')
const geometryColumn = toRef(props, 'geometryColumn')

// Every write bumps this: the map renders from tiles cached by URL, so an
// edited geometry only appears once the layers are told the data changed
// (they would otherwise wait for the next zoom to refetch).
const dataVersion = ref(0)

const toast = useToast()
const filter = useRowFilter(tableId, columns, isGeo)
const selection = props.selection

// The table's own effective row limit (TODO-F43) — PAGE_SIZE only until it
// has loaded, so a raised per-table or global limit is what the grid pages
// on rather than the client's own constant.
// rowlimit LIMITATION if high value could decrease front performance...
const tableSettings = useTableSettings(tableId)
const rowLimit = computed(
    () => tableSettings.settings.value?.defaultRowLimit.effective ?? PAGE_SIZE
)

const rowsState = useTableRows(
    tableId,
    toRef(props, 'rowColumns'),
    geometryColumn,
    () => {
        dataVersion.value += 1
        emit('mutated')
    },
    rowLimit,
    props.initialSort
)
watch(rowsState.sorting, (sort) => emit('update:sort', sort), { deep: true })
const rowEdits = useRowEdits(columns, toRef(props, 'rowColumns'))
const rowGeometry = useRowGeometry(tableId)
const layerEditing = useLayerEditing(tableId, geometryColumn, async () => {
    dataVersion.value += 1
    await rowsState.retry()
    emit('mutated')
})

// Editing is **one** mode of the table, not two: the rows and the geometries
// are two halves of the same object, and asking the user which half they mean
// before they have touched anything is asking a question they cannot answer
// yet. What the mode offers is what the table can carry — a table without
// geometry simply has no geometry tools (§8.8).
const editing = computed(() => rowEdits.active.value || layerEditing.active.value)

function toggleEditing() {
    if (editing.value) {
        cancelEditing()
        return
    }
    rowEdits.start()
    if (props.isGeo) layerEditing.start()
}

// Leaving the mode drops both halves at once — the reshapes and the retyped
// cells are one session. Cancelling says what it drops: a session opened to
// give a new row its geometry loses that row (useLayerEditing.cancel).
function cancelEditing() {
    layerEditing.cancel()
    rowEdits.stop()
}

// One *Enregistrer* for the whole session, wherever its button stands: the
// geometries the map committed and the cells the grid holds are written
// together, and the mode only closes if the write landed.
const NO_GEOMETRY_CHANGES: LayerChanges = { updates: new Map(), inserts: [] }

async function saveEditing(changes: LayerChanges = NO_GEOMETRY_CHANGES) {
    // A cell whose text does not fit its column is refused here, where the grid
    // can point at it, rather than by the server for the whole session at once.
    const invalid = rowEdits.invalidCount.value
    if (invalid > 0) {
        toast.error(
            `${invalid} valeur(s) ne correspondent pas au type de leur colonne : corrigez-les avant d'enregistrer.`
        )
        return
    }
    if (
        await layerEditing.save(
            changes,
            rowEdits.pendingUpdates.value,
            rowEdits.pendingInserts.value
        )
    )
        rowEdits.stop()
}

// The tools the map owns are rendered into the bar above both panels: they
// are part of the same mode as the row tools, and they have to stay reachable
// when the map is the hidden half (see mapMounted).
const MAP_TOOLS_ID = 'darow-map-edit-tools'

// Table and map share the space 50/50 by default; either can be maximized to
// take the whole width, restored back to 'split' from the same toggle group.
// Neither edit mode touches it: the layout is the user's, and an editing
// session that reframes the screen under them takes away the only view they
// were working from (§8.3).
const focusPanel = ref<'split' | 'table' | 'map'>('split')
watch(tableId, () => (focusPanel.value = 'split'))

// Re-centers the map on the filter's own results rather than the whole
// table's extent, cleared back to the table's bbox once inactive.
const mapBbox = computed(() => filter.filteredBbox.value ?? props.bbox)

// Creating a row is one gesture, and on a geo table it is a **drawing**: the
// shape closed on the map raises the empty row it belongs to, at the top of
// the grid, where its attributes are typed like any others. A table with no
// geometry has nothing to draw, so its *Créer* is a plain `+`.
function addDraftRow() {
    rowEdits.addDraft()
}

const displayRows = computed(() => {
    // The rows the session is about to create come first: they are empty, and
    // a row that has no value to sort on has no place among the sorted ones.
    const drafts = rowEdits.draftRows.value
    if (selection.selectedRows.value.length === 0) return [...drafts, ...rowsState.rows.value]
    const selectedSet = new Set(selection.selectedRows.value.map((row) => row.pkey))
    return [
        ...drafts,
        ...selection.selectedRows.value,
        ...rowsState.rows.value.filter((row) => !selectedSet.has(row.pkey))
    ]
})

function rowClassFor(row: { original: Row }) {
    // Violet, the colour a shape that belongs to no row yet is drawn in on the
    // map: the same object, the same state, on both halves of the screen.
    if (isDraft(row.original.pkey)) return 'row-draft'
    if (!props.isGeo) return ''
    return selection.selectedPkeys.value.includes(row.original.pkey)
        ? 'row-selected cursor-pointer'
        : 'cursor-pointer'
}

// One click on a row toggles it in and out of the selection, exactly as one
// click on its feature does — map selection and table selection are the same
// selection, so the gesture that reverses it has to be the same too (§8.8).
// A click that landed in a cell's field belongs to the field, not to the row:
// DataTable has already filtered those out, so what reaches here is a click on
// the row itself.
function onRowClick(row: { original: Row }) {
    // A row that does not exist yet cannot be selected: there is nothing on the
    // map to show, and its own geometry is the shape being drawn.
    if (!props.isGeo || isDraft(row.original.pkey)) return
    selection.toggle(row.original.pkey)
}

// Opened from the grid's own column header, not from TableHeader: it acts on
// one column of the table in hand, the same register as GeometryModal below.
const transformColumn = ref<string | null>(null)
const transformColumnSchema = computed(
    () => columns.value.find((c) => c.name === transformColumn.value) ?? null
)

// The write happens outside useTableRows/useLayerEditing, so it reloads
// through the exact path their own writes already trigger: bump
// dataVersion (the map's tiles are cached by URL, and the transform bumped
// UpdatedAt), reload the current page, and let the parent refresh the
// table's metadata.
async function onColumnTransformed() {
    dataVersion.value += 1
    await rowsState.retry()
    emit('mutated')
}
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-4">
        <TableToolbar
            :table-id="tableId"
            :columns="columns"
            :is-geo="isGeo"
            :editing="editing"
            :focus-panel="focusPanel"
            :active-filter-id="activeFilterId"
            @update:nodes="filter.conditions.value = $event"
            @update:focus-panel="focusPanel = $event"
            @update:active-filter-id="emit('update:activeFilterId', $event)"
            @toggle-editing="toggleEditing"
            @open-columns="emit('openColumns')"
        />

        <TableEditBar
            v-if="editing"
            :is-geo="isGeo"
            :tools-id="MAP_TOOLS_ID"
            :row-count="rowEdits.dirtyCount.value"
            :draft-count="rowEdits.draftCount.value"
            :saving="layerEditing.saving.value"
            @add-row="addDraftRow"
            @save="saveEditing()"
            @cancel="cancelEditing"
        />

        <div class="flex min-h-0 flex-1 gap-4">
            <div
                v-if="!isGeo || focusPanel !== 'map'"
                class="flex min-h-0 flex-col"
                :class="isGeo ? (focusPanel === 'table' ? 'w-full' : 'w-1/2') : 'w-full flex-1'"
            >
                <FilteredRowsPanel
                    v-if="filter.filterActive.value"
                    key="filter"
                    :visible-columns="visibleColumns"
                    :rows="filter.filterResults.value"
                    :loading="filter.filtering.value"
                    :error="filter.filterError.value"
                    :limit="filter.filterLimit.value"
                    :is-geo="isGeo"
                    :geometry-column="geometryColumn"
                    :row-class="rowClassFor"
                    @update:limit="filter.filterLimit.value = $event"
                    @retry="filter.retry"
                    @row-click="onRowClick"
                    @geometry="rowGeometry.open"
                />
                <TableRowsPanel
                    v-else
                    key="rows"
                    :state="rowsState"
                    :edits="rowEdits"
                    :editable-columns="rowColumns"
                    :columns="columns"
                    :display-rows="displayRows"
                    :visible-columns="visibleColumns"
                    :is-geo="isGeo"
                    :geometry-column="geometryColumn"
                    :row-class="rowClassFor"
                    @geometry="rowGeometry.open"
                    @row-click="onRowClick"
                    @transform="transformColumn = $event"
                />
            </div>

            <TableMapPanel
                :table-id="tableId"
                :bbox="mapBbox"
                :is-geo="isGeo"
                :focus-panel="focusPanel"
                :conditions="filter.appliedConditions.value"
                :selected-pkeys="selection.selectedPkeys.value"
                :editing="layerEditing.active.value"
                :features="layerEditing.features.value"
                :saving="layerEditing.saving.value"
                :row-count="rowEdits.dirtyCount.value"
                :toolbar-target="`#${MAP_TOOLS_ID}`"
                :data-version="dataVersion"
                :zoom-bbox="selection.zoomBbox.value"
                :zoom-token="selection.zoomToken.value"
                @toggle-selection="selection.toggle"
                @request-feature="layerEditing.reach"
                @release-features="layerEditing.release"
                @drawn-count="rowEdits.syncDrafts"
                @save="saveEditing"
                @cancel="cancelEditing"
            />
        </div>
    </div>

    <GeometryModal
        v-if="rowGeometry.pkey.value"
        :pkey="rowGeometry.pkey.value"
        :geometry="rowGeometry.geometry.value"
        :wkt="rowGeometry.wkt.value"
        :loading="rowGeometry.loading.value"
        :error="rowGeometry.error.value"
        @close="rowGeometry.close"
        @retry="rowGeometry.retry"
        @copy="rowGeometry.copy"
    />

    <TransformColumnModal
        v-if="transformColumnSchema"
        :table-id="tableId"
        :column="transformColumnSchema"
        :columns="columns"
        :geometry-column="geometryColumn"
        @close="transformColumn = null"
        @transformed="onColumnTransformed"
    />
</template>
