import { ref } from 'vue'
import { useAsyncAction } from './useAsyncAction'
import { useToast } from './useToast'

// The mutations behind ColumnStructureModal: rename and drop, on the writes
// the caller's useTableMetadata instance owns (onRename/onDrop), as
// useAddColumn is for AddColumnModal (§0.5, §0.12). busyColumn stays its own
// concept rather than a single isLoading — it is what §8.2 requires to show
// which row of the list is being written, distinct from useAsyncAction's own
// (unread) loading flag.
export function useColumnStructure(
    onRename: (name: string, newName: string) => Promise<void>,
    onDrop: (name: string) => Promise<void>
) {
    const toast = useToast()
    const editing = ref<string | null>(null)
    const draftName = ref('')
    const confirmingDrop = ref<string | null>(null)
    const busyColumn = ref<string | null>(null)
    const { error, run } = useAsyncAction()

    function startRename(col: string) {
        editing.value = col
        draftName.value = col
        error.value = ''
    }

    async function confirmRename(col: string) {
        const name = draftName.value.trim()
        if (!name || name === col) {
            editing.value = null
            return
        }
        busyColumn.value = col
        const result = await run(() => onRename(col, name), 'Impossible de renommer la colonne')
        busyColumn.value = null
        if (result === null) return
        editing.value = null
        toast.success('Colonne renommée.')
    }

    async function confirmDrop(col: string) {
        busyColumn.value = col
        const result = await run(() => onDrop(col), 'Impossible de supprimer la colonne')
        busyColumn.value = null
        confirmingDrop.value = null
        if (result === null) return
        toast.success('Colonne supprimée.')
    }

    return {
        editing,
        draftName,
        confirmingDrop,
        busyColumn,
        error,
        startRename,
        confirmRename,
        confirmDrop
    }
}
