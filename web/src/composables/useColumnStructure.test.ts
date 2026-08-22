import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useColumnStructure } from './useColumnStructure'
import { useToast } from './useToast'

function setup(
    onRename = vi.fn().mockResolvedValue(undefined),
    onDrop = vi.fn().mockResolvedValue(undefined)
) {
    return { onRename, onDrop, ...useColumnStructure(onRename, onDrop) }
}

describe('useColumnStructure', () => {
    beforeEach(() => {
        useToast().toasts.value = []
    })

    it('opens a rename on the column’s current name', () => {
        const { editing, draftName, startRename } = setup()

        startRename('name')

        expect(editing.value).toBe('name')
        expect(draftName.value).toBe('name')
    })

    it('clears a previous error when a rename is opened', async () => {
        const { error, startRename, draftName, confirmRename } = setup(
            vi.fn().mockRejectedValue(new Error('boom'))
        )
        startRename('name')
        draftName.value = 'nom'
        await confirmRename('name')

        startRename('height')

        expect(error.value).toBe('')
    })

    it('renames on the trimmed draft and confirms with a toast', async () => {
        const { onRename, draftName, editing, startRename, confirmRename } = setup()
        const { toasts } = useToast()
        startRename('name')
        draftName.value = '  nom  '

        await confirmRename('name')

        expect(onRename).toHaveBeenCalledWith('name', 'nom')
        expect(editing.value).toBeNull()
        expect(toasts.value[0].message).toBe('Colonne renommée.')
    })

    it('writes nothing when the name is blank or unchanged', async () => {
        const { onRename, draftName, editing, startRename, confirmRename } = setup()
        startRename('name')

        await confirmRename('name')
        draftName.value = '   '
        await confirmRename('name')

        expect(onRename).not.toHaveBeenCalled()
        expect(editing.value).toBeNull()
    })

    it('keeps the rename open on a refusal, stating it inline', async () => {
        const { editing, error, draftName, startRename, confirmRename } = setup(
            vi.fn().mockRejectedValue(new Error('nom déjà pris'))
        )
        const { toasts } = useToast()
        startRename('name')
        draftName.value = 'nom'

        await confirmRename('name')

        expect(editing.value).toBe('name')
        expect(error.value).toBe('Impossible de renommer la colonne : nom déjà pris')
        expect(toasts.value).toEqual([])
    })

    it('names the column being written, then releases it', async () => {
        const { busyColumn, draftName, startRename, confirmRename } = setup(
            vi.fn(async () => {
                expect(busyColumn.value).toBe('name')
            })
        )
        startRename('name')
        draftName.value = 'nom'

        await confirmRename('name')

        expect(busyColumn.value).toBeNull()
    })

    it('drops a column and confirms with a toast', async () => {
        const { onDrop, confirmingDrop, confirmDrop } = setup()
        const { toasts } = useToast()
        confirmingDrop.value = 'name'

        await confirmDrop('name')

        expect(onDrop).toHaveBeenCalledWith('name')
        expect(confirmingDrop.value).toBeNull()
        expect(toasts.value[0].message).toBe('Colonne supprimée.')
    })

    it('states a refused drop inline, with no toast', async () => {
        const { error, confirmDrop } = setup(
            undefined,
            vi.fn().mockRejectedValue(new Error('boom'))
        )
        const { toasts } = useToast()

        await confirmDrop('name')

        expect(error.value).toBe('Impossible de supprimer la colonne : boom')
        expect(toasts.value).toEqual([])
    })
})
