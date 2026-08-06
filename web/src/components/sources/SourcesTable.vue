<script setup lang="ts">
import { computed, h } from 'vue'
import {
    createColumnHelper,
    getCoreRowModel,
    getExpandedRowModel,
    useVueTable
} from '@tanstack/vue-table'
import { layerKey, useSources } from '../../composables/useSources'
import { toSourceRows } from '../../utils/sourceRows'
import type { SourceRow } from '../../utils/sourceRows'
import Badge from '../ui/Badge.vue'
import DataTable from '../ui/DataTable.vue'
import ErrorState from '../ui/ErrorState.vue'
import ListFooter from '../ui/ListFooter.vue'
import Skeleton from '../ui/Skeleton.vue'
import SourceActionsCell from './SourceActionsCell.vue'
import SourceNameCell from './SourceNameCell.vue'
import SourceSizeCell from './SourceSizeCell.vue'

// The files/layers tree: column definitions, the TanStack instance and the
// four states (§8.1). Takes the whole useSources instance rather than a
// narrower prop set — its action functions (startLoad, remove…) are already
// the single owner of that state, so the cell definitions call them directly
// the way TableRowsPanel calls into useTableRows.
const props = defineProps<{ state: ReturnType<typeof useSources> }>()

const rows = computed<SourceRow[]>(() => toSourceRows(props.state.sources.value))

function rowKey(row: SourceRow): string {
    return row.layerName ? layerKey(row.source, row.layerName) : row.source.name
}

const columnHelper = createColumnHelper<SourceRow>()
const columnDefs = [
    columnHelper.accessor('name', {
        header: 'Nom',
        cell: ({ row, getValue }) =>
            h(SourceNameCell, {
                name: getValue(),
                isLayer: row.depth > 0,
                layerCount: row.original.layerCount,
                ext: row.original.source.ext
            })
    }),
    columnHelper.display({
        id: 'size',
        header: 'Taille',
        enableSorting: true,
        cell: ({ row }) =>
            h(SourceSizeCell, {
                sizeBytes: row.original.sizeBytes,
                layerCount: row.original.layerCount
            })
    }),
    columnHelper.display({
        id: 'status',
        header: 'Statut',
        enableSorting: true,
        cell: ({ row }) =>
            row.original.layerCount !== undefined
                ? null
                : h(Badge, { tone: row.original.loaded ? 'success' : 'neutral' }, () =>
                      row.original.loaded ? 'Chargée' : 'Non chargée'
                  )
    }),
    columnHelper.display({
        id: 'actions',
        header: '',
        meta: { align: 'right' },
        cell: ({ row }) =>
            h(SourceActionsCell, {
                row: row.original,
                editingEpsg: props.state.epsgEditingKey.value === rowKey(row.original),
                epsg: props.state.epsgInput.value,
                detecting: props.state.epsgDetecting.value,
                epsgApplies: props.state.epsgApplies.value,
                loadingRow: props.state.loadingKey.value === rowKey(row.original),
                deleting: props.state.deletingName.value === row.original.source.name,
                'onUpdate:epsg': (value: string) => (props.state.epsgInput.value = value),
                onStartLoad: () =>
                    props.state.startLoad(row.original.source, row.original.layerName),
                onCancelLoad: props.state.cancelLoad,
                onConfirmLoad: () =>
                    props.state.confirmLoad(row.original.source, row.original.layerName),
                onRemove: () => props.state.remove(row.original.source.name)
            })
    })
]

const table = useVueTable({
    get data() {
        return rows.value
    },
    columns: columnDefs,
    getRowId: (row, _index, parent) => (parent ? `${parent.id}::${row.name}` : row.name),
    getCoreRowModel: getCoreRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    getSubRows: (row) => row.subRows,
    manualSorting: true,
    state: {
        expanded: true,
        get sorting() {
            return props.state.sorting.value
        }
    },
    onSortingChange: props.state.setSorting
})
</script>

<template>
    <Skeleton v-if="state.loading.value && !state.loadedOnce.value" />
    <ErrorState v-else-if="state.error.value" :message="state.error.value" @retry="state.retry" />
    <DataTable
        v-else
        :table="table"
        :loading="state.loading.value"
        empty-message="Aucune source détectée — importez un fichier pour commencer."
    >
        <template #footer>
            <ListFooter :sentinel="state.sentinel" :loading="state.loading.value" />
        </template>
    </DataTable>
</template>
