<script setup lang="ts">
import { computed, h } from 'vue'
import type { ColumnDef } from '@tanstack/vue-table'
import { createColumnHelper, getCoreRowModel, useVueTable } from '@tanstack/vue-table'
import type { ColumnSchema, Row, RowValue } from '../../client/types'
import { isDraft } from '../../composables/useRowEdits'
import type { useRowEdits } from '../../composables/useRowEdits'
import type { useTableRows } from '../../composables/useTableRows'
import { columnType } from '../../utils/columnTypes'
import { cellValue } from '../../utils/rows'
import DataTable from '../ui/DataTable.vue'
import TableColumnHeaderCell from './TableColumnHeaderCell.vue'
import TableGeometryCell from './TableGeometryCell.vue'
import TableRowLeadCell from './TableRowLeadCell.vue'
import TableRowEditCell from './TableRowEditCell.vue'
import ErrorState from '../ui/ErrorState.vue'
import ListFooter from '../ui/ListFooter.vue'
import Skeleton from '../ui/Skeleton.vue'

// The rows grid of a table: the rows as its parent arranged them, the infinite scroll, and
// the cells the attribute edit mode makes writable. It receives the row state
// its view already owns (useTableRows) and reports intentions; it loads
// nothing itself, and it does not own the mode either — arming it is a
// table-level action, next to the one that arms the geometries (§8.8).
const props = defineProps<{
    state: ReturnType<typeof useTableRows>
    // The session's attribute drafts. Which columns it may write is not the
    // grid's to guess: the geometry is drawn, never typed, and a column the
    // table does not own (the pkey) is not a field either.
    edits: ReturnType<typeof useRowEdits>
    editableColumns: string[]
    columns: ColumnSchema[]
    displayRows: Row[]
    // The grid is showing the selection alone: its foot states what is on
    // screen, and no sentinel — there is no next page of a selection, and the
    // paged count would name a total the grid is not showing (§8.6).
    selectionOnly?: boolean
    visibleColumns: string[]
    isGeo: boolean
    // The geometry column's own name: it heads the column that opens it.
    geometryColumn?: string
    rowClass?: (row: { original: Row }) => string
}>()
const emit = defineEmits<{
    geometry: [pkey: string]
    rowClick: [row: { original: Row }]
    transform: [column: string]
}>()

const columnHelper = createColumnHelper<Row>()
const columnDefs = computed(() => {
    const defs: ColumnDef<Row, RowValue>[] = props.visibleColumns.map((col) =>
        columnHelper.accessor((row) => cellValue(row, col), {
            id: col,
            header: () =>
                h(TableColumnHeaderCell, { name: col, onTransform: () => emit('transform', col) }),
            cell: (info) => {
                const row = info.row.original
                if (props.edits.active.value && props.editableColumns.includes(col)) {
                    return h(TableRowEditCell, {
                        modelValue: props.edits.display(row, col),
                        column: col,
                        type: columnType(props.columns, col),
                        dirty: props.edits.isDirty(row.pkey, col),
                        'onUpdate:modelValue': (value: string) =>
                            props.edits.setCell(row, col, value)
                    })
                }
                const v = info.getValue()
                return v === null || v === undefined ? '' : String(v)
            }
        })
    )

    // Its width is stated rather than fitted, and so is the offset of whatever
    // is pinned after it: both depend on the session, which is what raises the
    // drafts whose one control shares that cell with the rank.
    const editing = props.edits.active.value

    // The geometry is a column of the table, so it is one of the grid: headed by
    // its own name, pinned next to the row's cell, holding the one control that
    // reads it (LIM-F3). A draft has none to read yet — its shape is the one
    // being drawn.
    if (props.isGeo) {
        defs.unshift(
            columnHelper.display({
                id: '__geometry',
                header: props.geometryColumn ?? 'geom',
                meta: {
                    pinned: editing ? 'left-20' : 'left-12',
                    class: 'w-16 max-w-16 px-2 truncate'
                },
                cell: (info) =>
                    isDraft(info.row.original.pkey)
                        ? null
                        : h(TableGeometryCell, {
                              onOpen: () => emit('geometry', info.row.original.pkey)
                          })
            })
        )
    }

    // The row's own cell, first and pinned: what names the row, plus the one
    // control a draft carries — neither is a value of the table. It stays put
    // while the columns scroll sideways, or a grid wider than its panel hides
    // the number that names a row. Duplicating and deleting are not here: they
    // act on the selection, from the bar above the grid (§8.6).
    defs.unshift(
        columnHelper.display({
            id: '__row',
            header: '#',
            meta: {
                align: 'right',
                pinned: 'left-0',
                class: editing
                    ? 'w-20 min-w-20 px-2 whitespace-nowrap'
                    : 'w-12 min-w-12 px-2 whitespace-nowrap'
            },
            cell: (info) => {
                const row = info.row.original
                // Dropping a draft is the one action that applies to it, and only
                // where the grid alone created it: a draft raised by a drawing goes
                // with its shape, which is the session's *Annuler*.
                if (isDraft(row.pkey)) {
                    return h(TableRowLeadCell, {
                        rank: null,
                        draft: !props.isGeo,
                        onDelete: () => props.edits.removeDraft(row.pkey)
                    })
                }
                return h(TableRowLeadCell, {
                    rank: info.row.index + 1 - props.edits.draftCount.value
                })
            }
        })
    )

    return defs
})

const table = useVueTable({
    get data() {
        return props.displayRows
    },
    get columns() {
        return columnDefs.value
    },
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    get pageCount() {
        return Math.ceil(props.state.total.value / props.state.pageSize.value)
    },
    state: {
        get sorting() {
            return props.state.sorting.value
        }
    },
    onSortingChange: props.state.setSorting
})
</script>

<template>
    <Skeleton v-if="state.loading.value && !state.loadedOnce.value" :rows="8" />
    <ErrorState v-else-if="state.error.value" :message="state.error.value" @retry="state.retry" />
    <DataTable
        v-else
        :table="table"
        :loading="state.loading.value"
        :row-class="rowClass"
        empty-message="Aucune ligne."
        @row-click="emit('rowClick', $event)"
    >
        <template #footer>
            <p v-if="selectionOnly" class="px-4 py-3 text-center text-xs text-base-content/40">
                Sélection seule — {{ displayRows.length.toLocaleString('fr-FR') }} sur
                {{ state.total.value.toLocaleString('fr-FR') }}
            </p>
            <ListFooter
                v-else
                :sentinel="state.sentinel"
                :loading="state.loading.value"
                :loaded="state.rows.value.length"
                :total="state.total.value"
            />
        </template>
    </DataTable>
</template>
