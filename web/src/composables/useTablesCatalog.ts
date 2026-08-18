import { ref } from 'vue'
import type { SortingState } from '@tanstack/vue-table'
import { deleteTable, listTables, renameTable } from '../client/tables'
import type { Metadata } from '../client/types'
import { errorMessage } from '../client/http'
import { LIST_PAGE_SIZE } from '../utils/pageSizes'
import { usePaginatedList } from './usePaginatedList'
import { useToast } from './useToast'

// The table catalogue and what can be done to a row from it: renaming one and
// deleting one, the two actions a table's own page already offers.
export function useTablesCatalog(initialSorting?: SortingState) {
    const toast = useToast()

    const list = usePaginatedList<Metadata>({
        limit: LIST_PAGE_SIZE,
        initialSorting,
        fetchPage: (offset, limit, sort) =>
            listTables(offset, limit, sort).then((page) => ({
                items: page.tables,
                total: page.total
            })),
        describeError: (e) => errorMessage(e, 'Impossible de charger les tables')
    })

    const editingId = ref<string | null>(null)
    const renamingId = ref<string | null>(null)
    const renameError = ref<{ tableId: string; message: string } | null>(null)
    const removingId = ref<string | null>(null)

    function startRename(tableId: string) {
        editingId.value = tableId
        renameError.value = null
    }

    function cancelRename() {
        editingId.value = null
        renameError.value = null
    }

    // Patches the renamed row in place instead of reloading the whole list: a
    // rename from this page must not lose the operator's current scroll
    // position, page or search (§8.6). A rejected name leaves the row in
    // edition, with the message under the field (§8.4).
    async function rename(tableId: string, name: string) {
        renamingId.value = tableId
        renameError.value = null
        try {
            const renamed = await renameTable(tableId, name)
            const index = list.items.value.findIndex((t) => t.tableId === renamed.tableId)
            if (index !== -1) list.items.value[index] = renamed
            editingId.value = null
        } catch (e) {
            renameError.value = {
                tableId,
                message: errorMessage(e, 'Impossible de renommer la table')
            }
        } finally {
            renamingId.value = null
        }
    }

    // Same in-place reasoning as rename: the deleted row leaves the list and
    // the total follows it, rather than the page resetting under the user.
    async function remove(tableId: string, tableName: string) {
        removingId.value = tableId
        try {
            await deleteTable(tableId)
            list.items.value = list.items.value.filter((t) => t.tableId !== tableId)
            list.total.value = Math.max(list.total.value - 1, 0)
            toast.success(`Table « ${tableName} » supprimée.`)
        } catch (e) {
            toast.error(errorMessage(e, `Impossible de supprimer la table « ${tableName} »`))
        } finally {
            removingId.value = null
        }
    }

    return {
        ...list,
        tables: list.items,
        editingId,
        renamingId,
        renameError,
        removingId,
        startRename,
        cancelRename,
        rename,
        remove
    }
}
