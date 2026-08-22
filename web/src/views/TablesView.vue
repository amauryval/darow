<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { Row } from '@tanstack/vue-table'
import {
    createColumnHelper,
    getCoreRowModel,
    getFilteredRowModel,
    useVueTable
} from '@tanstack/vue-table'
import { GitBranch } from '@lucide/vue'
import type { Metadata } from '../client/types'
import { useTablesCatalog } from '../composables/useTablesCatalog'
import { useDocumentTitle } from '../composables/useDocumentTitle'
import { useWriteGuard } from '../composables/useWriteGuard'
import { parseSorting, stringifySorting } from '../utils/sort'
import { queryString } from '../utils/routeQuery'
import Badge from '../components/ui/Badge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import DataTable from '../components/ui/DataTable.vue'
import ErrorState from '../components/ui/ErrorState.vue'
import ListFooter from '../components/ui/ListFooter.vue'
import Skeleton from '../components/ui/Skeleton.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import Breadcrumbs from '../components/ui/Breadcrumbs.vue'
import TableActionsCell from '../components/tables/TableActionsCell.vue'
import TableNameCell from '../components/tables/TableNameCell.vue'
import DeriveSqlModal from '../components/tables/DeriveSqlModal.vue'

const router = useRouter()
const route = useRoute()

useDocumentTitle(computed(() => 'Tables'))

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

const list = useTablesCatalog(
    queryString(route.query.sort)
        ? parseSorting(queryString(route.query.sort))
        : [{ id: 'updatedAt', desc: true }]
)
const { tables, total, loading, loadedOnce, error, sorting, setSorting, loadMore, retry } = list

const globalFilter = ref(queryString(route.query.q))

watch([globalFilter, sorting], ([q, sort]) => {
    router.replace({
        query: {
            ...route.query,
            q: q.trim() || undefined,
            sort: stringifySorting(sort) || undefined
        }
    })
})

// When search is active, load all remaining pages to filter across the full
// dataset: client-side filtering on partial pages violates §8.6 (a silent
// partial result is a lie).
watch(globalFilter, async (filter) => {
    if (filter.trim() && tables.value.length < total.value) {
        await loadMore()
    }
})

function onRowClick(row: Row<Metadata>) {
    router.push(`/tables/${row.original.tableId}`)
}

// Free-SQL derivation acts on the whole catalogue, not one table's page, so
// it opens from here. The created table is what the user
// wants to see next, the way a source's upload already navigates there.
const showDeriveSql = ref(false)

async function onDerived(result: { tableId: string }) {
    showDeriveSql.value = false
    await router.push(`/tables/${result.tableId}`)
}

const columnHelper = createColumnHelper<Metadata>()
const columnDefs = [
    columnHelper.accessor('tableName', {
        header: 'Nom',
        cell: ({ row }) =>
            h(TableNameCell, {
                tableId: row.original.tableId,
                tableName: row.original.tableName,
                editing: list.editingId.value === row.original.tableId,
                renaming: list.renamingId.value === row.original.tableId,
                error:
                    list.renameError.value?.tableId === row.original.tableId
                        ? list.renameError.value.message
                        : undefined,
                onRename: list.rename,
                onCancel: list.cancelRename
            })
    }),
    columnHelper.display({
        id: 'type',
        header: 'Type',
        enableGlobalFilter: false,
        enableSorting: false,
        cell: ({ row }) =>
            h(Badge, { tone: row.original.isGeo ? 'primary' : 'neutral' }, () =>
                row.original.isGeo ? 'Géo' : 'Standard'
            )
    }),
    columnHelper.accessor('rowCount', {
        header: 'Lignes',
        enableGlobalFilter: false,
        cell: ({ getValue }) => getValue().toLocaleString('fr-FR')
    }),
    columnHelper.accessor('updatedAt', {
        header: 'Mise à jour',
        enableGlobalFilter: false,
        cell: ({ getValue }) => new Date(getValue()).toLocaleString('fr-FR')
    }),
    columnHelper.display({
        id: 'actions',
        header: '',
        enableGlobalFilter: false,
        meta: { align: 'right' },
        cell: ({ row }) =>
            h(TableActionsCell, {
                tableName: row.original.tableName,
                removing: list.removingId.value === row.original.tableId,
                onRename: () => list.startRename(row.original.tableId),
                onRemove: () => list.remove(row.original.tableId, row.original.tableName)
            })
    })
]

const table = useVueTable({
    get data() {
        return tables.value
    },
    columns: columnDefs,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    manualSorting: true,
    state: {
        get sorting() {
            return sorting.value
        },
        get globalFilter() {
            return globalFilter.value
        }
    },
    onSortingChange: setSorting,
    onGlobalFilterChange: (updater) => {
        globalFilter.value = typeof updater === 'function' ? updater(globalFilter.value) : updater
    }
})
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
        <Breadcrumbs :items="[{ label: 'Tables disponibles' }]" />
        <div class="flex shrink-0 items-center justify-between">
            <BaseInput v-model="globalFilter" placeholder="Rechercher une table…" class="w-56" />
            <BaseButton
                variant="secondary"
                size="sm"
                :icon="GitBranch"
                :disabled="writeDisabled"
                :title="writeDisabled ? writeGuardReason : undefined"
                @click="showDeriveSql = true"
                >Créer une nouvelle table</BaseButton
            >
        </div>

        <Skeleton v-if="loading && !loadedOnce" />
        <ErrorState v-else-if="error" :message="error" @retry="retry" />
        <DataTable
            v-else
            :table="table"
            :row-class="() => 'cursor-pointer hover'"
            :loading="loading"
            :empty-message="
                tables.length === 0
                    ? 'Aucune table. Chargez une source depuis l\'onglet Sources.'
                    : 'Aucun résultat.'
            "
            @row-click="onRowClick"
        >
            <template #footer>
                <ListFooter
                    :sentinel="list.sentinel"
                    :loading="loading"
                    :loaded="tables.length"
                    :total="total"
                />
            </template>
        </DataTable>

        <DeriveSqlModal v-if="showDeriveSql" @close="showDeriveSql = false" @derived="onDerived" />
    </div>
</template>
