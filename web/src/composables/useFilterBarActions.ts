import type { FilterCondition, SavedFilter } from '../client/types'
import { useAsyncAction } from './useAsyncAction'
import type { useSavedFilters } from './useSavedFilters'
import { useToast } from './useToast'

// The save/remove pair TableFilterBar drives over useSavedFilters: create or
// update depending on whether the editor was opened on an existing filter,
// and delete. The editor's own error stays inline rather than a toast — the
// form is still in front of the user and the conditions they typed are still
// there to fix (§8.4) — while a removal, which opens no form, toasts either
// way.
export function useFilterBarActions(saved: ReturnType<typeof useSavedFilters>) {
    const toast = useToast()
    const { error: editorError, run: runSubmit } = useAsyncAction()
    const { error: removeError, run: runRemove } = useAsyncAction()

    async function submit(
        target: SavedFilter | null,
        name: string,
        conditions: FilterCondition[]
    ): Promise<SavedFilter | null> {
        const stored = await runSubmit(
            () =>
                target
                    ? saved.update(target.filterId, { name, conditions })
                    : saved.create(name, conditions),
            "Impossible d'enregistrer le filtre"
        )
        if (stored === null) return null
        toast.success(target ? `Filtre « ${name} » mis à jour.` : `Filtre « ${name} » enregistré.`)
        return stored
    }

    async function remove(filter: SavedFilter): Promise<boolean> {
        const result = await runRemove(
            () => saved.remove(filter.filterId),
            'Impossible de supprimer le filtre'
        )
        if (result === null) {
            toast.error(removeError.value)
            return false
        }
        toast.success(`Filtre « ${filter.name} » supprimé.`)
        return true
    }

    return { editorError, submit, remove }
}
