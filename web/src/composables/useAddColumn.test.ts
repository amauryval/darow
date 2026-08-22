import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAddColumn } from './useAddColumn'
import { useToast } from './useToast'

describe('useAddColumn', () => {
    beforeEach(() => {
        useToast().toasts.value = []
    })

    it('trims what it hands to the write', async () => {
        const onAdd = vi.fn().mockResolvedValue(undefined)

        await useAddColumn(onAdd).confirm('  surface  ', ' DOUBLE ')

        expect(onAdd).toHaveBeenCalledWith('surface', 'DOUBLE')
    })

    it('refuses a blank name without calling the write', async () => {
        const onAdd = vi.fn()

        await expect(useAddColumn(onAdd).confirm('   ', 'DOUBLE')).resolves.toBe(false)
        expect(onAdd).not.toHaveBeenCalled()
    })

    it('confirms the addition with a toast', async () => {
        const { toasts } = useToast()

        await expect(useAddColumn(async () => {}).confirm('surface', 'DOUBLE')).resolves.toBe(true)

        expect(toasts.value).toEqual([
            expect.objectContaining({ kind: 'success', message: 'Colonne ajoutée.' })
        ])
    })

    it('keeps the refusal inline, with no toast', async () => {
        const { toasts } = useToast()
        const { error, confirm } = useAddColumn(() => Promise.reject(new Error('déjà présente')))

        await expect(confirm('surface', 'DOUBLE')).resolves.toBe(false)

        expect(error.value).toBe("Impossible d'ajouter la colonne : déjà présente")
        expect(toasts.value).toEqual([])
    })
})
