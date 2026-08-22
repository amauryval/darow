import { useAsyncAction } from './useAsyncAction'
import { useToast } from './useToast'

// The mutation behind AddColumnModal: the write is the caller's
// useTableMetadata instance (onAdd), passed in rather than called here, so
// the schema stays owned by one place (§0.5, §0.12) — this composable is
// only the loading/error/toast wrapping around it.
export function useAddColumn(onAdd: (name: string, type: string) => Promise<void>) {
    const toast = useToast()
    const { isLoading, error, run } = useAsyncAction()

    async function confirm(name: string, type: string): Promise<boolean> {
        const trimmedName = name.trim()
        if (!trimmedName) return false
        const result = await run(
            () => onAdd(trimmedName, type.trim()),
            "Impossible d'ajouter la colonne"
        )
        if (result === null) return false
        toast.success('Colonne ajoutée.')
        return true
    }

    return { isLoading, error, confirm }
}
