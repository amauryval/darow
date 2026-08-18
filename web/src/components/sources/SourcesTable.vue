<script setup lang="ts">
import { computed, h } from 'vue'
import {
    createColumnHelper,
    getCoreRowModel,
    getExpandedRowModel,
    useVueTable
} from '@tanstack/vue-table'
import { useSources } from '../../composables/useSources'
import type { useTableProvenance } from '../../composables/useTableProvenance'
import { toSourceRows } from '../../utils/sourceRows'
import type { SourceRow } from '../../utils/sourceRows'
import DataTable from '../ui/DataTable.vue'
import ErrorState from '../ui/ErrorState.vue'
import ListFooter from '../ui/ListFooter.vue'
import Skeleton from '../ui/Skeleton.vue'
import SourceActionsCell from './SourceActionsCell.vue'
import SourceNameCell from './SourceNameCell.vue'
import SourceTablesCell from './SourceTablesCell.vue'
import SourceSizeCell from './SourceSizeCell.vue'

// The files/layers tree: column definitions, the TanStack instance and the
// four states (§8.1). Takes the whole useSources instance — it already owns
// that state, so the cell definitions call its actions directly.
const props = defineProps<{
    state: ReturnType<typeof useSources>
    provenance: ReturnType<typeof useTableProvenance>
}>()

const emit = defineEmits<{ startLoad: [row: SourceRow] }>()

// A file row lists what the file itself produced; a layer row only what that
// layer did — the two never mix (§3.2 back).
function tablesOf(row: SourceRow) {
    return row.layerCount === undefined
        ? props.provenance.tablesOf(row.source.name, row.layerName)
        : []
}

const rows = computed<SourceRow[]>(() => toSourceRows(props.state.sources.value))

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
        id: 'tables',
        header: 'Tables chargées',
        cell: ({ row }) => h(SourceTablesCell, { tables: tablesOf(row.original) })
    }),
    columnHelper.display({
        id: 'actions',
        header: '',
        meta: { align: 'right' },
        cell: ({ row }) =>
            h(SourceActionsCell, {
                row: row.original,
                loadedCount: tablesOf(row.original).length,
                deleting: props.state.deletingName.value === row.original.source.name,
                onStartLoad: () => emit('startLoad', row.original),
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
            <ListFooter
                :sentinel="state.sentinel"
                :loading="state.loading.value"
                :loaded="state.sources.value.length"
                :total="state.total.value"
            />
        </template>
    </DataTable>
</template>
