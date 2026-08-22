import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useFilterBarActions } from './useFilterBarActions'
import { useToast } from './useToast'
import type { useSavedFilters } from './useSavedFilters'
import type { FilterCondition, SavedFilter } from '../client/types'

const conditions: FilterCondition[] = [{ column: 'name', operator: '=', value: 'Nantes' }]

const stored: SavedFilter = {
    filterId: 'f1',
    tableId: 't1',
    name: 'Nantes',
    conditions,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z'
}

function savedFilters(overrides: Partial<ReturnType<typeof useSavedFilters>> = {}) {
    return {
        create: vi.fn().mockResolvedValue(stored),
        update: vi.fn().mockResolvedValue(stored),
        remove: vi.fn().mockResolvedValue(undefined),
        ...overrides
    } as unknown as ReturnType<typeof useSavedFilters>
}

describe('useFilterBarActions', () => {
    beforeEach(() => {
        useToast().toasts.value = []
    })

    it('creates a filter when the editor was opened on none', async () => {
        const saved = savedFilters()
        const { toasts } = useToast()

        await expect(
            useFilterBarActions(saved).submit(null, 'Nantes', conditions)
        ).resolves.toEqual(stored)

        expect(saved.create).toHaveBeenCalledWith('Nantes', conditions)
        expect(toasts.value[0]).toMatchObject({
            kind: 'success',
            message: 'Filtre « Nantes » enregistré.'
        })
    })

    it('updates the filter the editor was opened on', async () => {
        const saved = savedFilters()
        const { toasts } = useToast()

        await useFilterBarActions(saved).submit(stored, 'Nantes 2', conditions)

        expect(saved.update).toHaveBeenCalledWith('f1', { name: 'Nantes 2', conditions })
        expect(toasts.value[0].message).toBe('Filtre « Nantes 2 » mis à jour.')
    })

    it('keeps a refused save inline — the form is still in front of the user', async () => {
        const saved = savedFilters({ create: vi.fn().mockRejectedValue(new Error('nom pris')) })
        const { toasts } = useToast()
        const { editorError, submit } = useFilterBarActions(saved)

        await expect(submit(null, 'Nantes', conditions)).resolves.toBeNull()

        expect(editorError.value).toBe("Impossible d'enregistrer le filtre : nom pris")
        expect(toasts.value).toEqual([])
    })

    it('confirms a removal with a toast naming the filter', async () => {
        const saved = savedFilters()
        const { toasts } = useToast()

        await expect(useFilterBarActions(saved).remove(stored)).resolves.toBe(true)

        expect(saved.remove).toHaveBeenCalledWith('f1')
        expect(toasts.value[0].message).toBe('Filtre « Nantes » supprimé.')
    })

    it('toasts a failed removal, which opens no form to carry it', async () => {
        const saved = savedFilters({ remove: vi.fn().mockRejectedValue(new Error('boom')) })
        const { toasts } = useToast()

        await expect(useFilterBarActions(saved).remove(stored)).resolves.toBe(false)

        expect(toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de supprimer le filtre : boom'
        })
    })
})
