import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useTableSettings } from './useTableSettings'
import { flush, withSetup } from '../test/fixtures'
import { useToast } from './useToast'
import { deleteTableSettings, getTableSettings, updateTableSettings } from '../client/tablesettings'
import type { OverrideField, TableSettings } from '../client/types'

vi.mock('../client/tablesettings', () => ({
    getTableSettings: vi.fn(),
    updateTableSettings: vi.fn(),
    deleteTableSettings: vi.fn()
}))
vi.mock('./useSettings', () => ({
    useSettings: () => ({ rowLimit: ref(100) })
}))

const read = vi.mocked(getTableSettings)
const patchSettings = vi.mocked(updateTableSettings)
const reset = vi.mocked(deleteTableSettings)

function rowLimitField(override: number | null, effective: number): OverrideField {
    return { unit: 'lignes', override, effective }
}

function settings(override: number | null, effective: number): TableSettings {
    return {
        tableId: 't1',
        isGeo: true,
        defaultRowLimit: rowLimitField(override, effective),
        updatedAt: '2026-01-01T00:00:00Z'
    }
}

function setup() {
    const tableId = ref('t1')
    const { result } = withSetup(() => useTableSettings(tableId))
    return { tableId, ...result }
}

describe('useTableSettings', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        read.mockResolvedValue(settings(500, 500))
    })

    it('loads the table’s overrides as soon as it is pointed at one', async () => {
        const { settings: loaded, loadedOnce, loading } = setup()
        expect(loading.value).toBe(true)

        await flush()

        expect(read).toHaveBeenCalledWith('t1')
        expect(loaded.value).toEqual(settings(500, 500))
        expect(loadedOnce.value).toBe(true)
        expect(loading.value).toBe(false)
    })

    it('reloads when the view moves to another table', async () => {
        const { tableId } = setup()
        await flush()

        tableId.value = 't2'
        await nextTick()
        await flush()

        expect(read).toHaveBeenLastCalledWith('t2')
    })

    it('shows a failed read rather than toasting past it', async () => {
        read.mockRejectedValue(new Error('boom'))
        const { error, loadedOnce, retry } = setup()
        await flush()

        expect(error.value).toBe('Impossible de charger les paramètres de la table : boom')
        expect(loadedOnce.value).toBe(false)
        expect(useToast().toasts.value).toEqual([])

        read.mockResolvedValue(settings(null, 100))
        await retry()
        expect(error.value).toBe('')
    })

    it('serves the table at its own override, and at the global row without one', async () => {
        const { rowLimit, settings: loaded } = setup()
        await flush()

        expect(rowLimit.value).toBe(500)

        loaded.value = null
        expect(rowLimit.value).toBe(100)
    })

    it('saves a patch and confirms with a toast', async () => {
        const { patch, settings: loaded, saving } = setup()
        await flush()
        patchSettings.mockResolvedValue(settings(200, 200))

        await expect(patch({ defaultRowLimit: 200 })).resolves.toBe(true)

        expect(patchSettings).toHaveBeenCalledWith('t1', { defaultRowLimit: 200 })
        expect(loaded.value?.defaultRowLimit.effective).toBe(200)
        expect(saving.value).toBe(false)
        expect(useToast().toasts.value[0].message).toBe('Paramètres de la table enregistrés.')
    })

    it('toasts a refused patch', async () => {
        const { patch } = setup()
        await flush()
        patchSettings.mockRejectedValue(new Error('hors bornes'))

        await expect(patch({ defaultRowLimit: 1 })).resolves.toBe(false)

        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: "Impossible d'enregistrer les paramètres de la table : hors bornes"
        })
    })

    it('resets every override back to the global row', async () => {
        const { resetAll, settings: loaded } = setup()
        await flush()
        reset.mockResolvedValue(settings(null, 100))

        await expect(resetAll()).resolves.toBe(true)

        expect(reset).toHaveBeenCalledWith('t1')
        expect(loaded.value?.defaultRowLimit.override).toBeNull()
        expect(useToast().toasts.value[0].message).toBe('Paramètres de la table réinitialisés.')
    })

    it('toasts a refused reset', async () => {
        const { resetAll } = setup()
        await flush()
        reset.mockRejectedValue(new Error('boom'))

        await expect(resetAll()).resolves.toBe(false)

        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de réinitialiser les paramètres de la table : boom'
        })
    })
})
