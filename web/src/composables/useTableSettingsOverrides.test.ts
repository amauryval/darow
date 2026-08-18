import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useTableSettingsOverrides } from './useTableSettingsOverrides'
import { flush } from '../test/fixtures'
import { useToast } from './useToast'
import { deleteTableSettings, listOverriddenTableSettings } from '../client/tablesettings'
import type { TableSettings } from '../client/types'

vi.mock('../client/tablesettings', () => ({
    listOverriddenTableSettings: vi.fn(),
    deleteTableSettings: vi.fn()
}))

const list = vi.mocked(listOverriddenTableSettings)
const reset = vi.mocked(deleteTableSettings)

function settings(tableId: string): TableSettings {
    return {
        tableId,
        isGeo: true,
        defaultRowLimit: { unit: 'lignes', override: 500, effective: 500 },
        updatedAt: '2026-01-01T00:00:00Z'
    }
}

describe('useTableSettingsOverrides', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        list.mockResolvedValue([settings('t1'), settings('t2')])
    })

    it('reads the dedicated route rather than sweeping the catalogue', async () => {
        const { tables, loadedOnce } = useTableSettingsOverrides()
        await flush()

        expect(list).toHaveBeenCalledTimes(1)
        expect(tables.value.map((t) => t.tableId)).toEqual(['t1', 't2'])
        expect(loadedOnce.value).toBe(true)
    })

    it('shows a failed read, retryable', async () => {
        list.mockRejectedValueOnce(new Error('boom'))
        const { error, loadedOnce, retry } = useTableSettingsOverrides()
        await flush()

        expect(error.value).toBe('Impossible de charger les dérogations par table : boom')
        expect(loadedOnce.value).toBe(false)

        await retry()

        expect(error.value).toBe('')
        expect(loadedOnce.value).toBe(true)
    })

    it('drops a reset table from the list without reloading it', async () => {
        const { tables, resetTable, resettingTableId } = useTableSettingsOverrides()
        await flush()
        reset.mockResolvedValue(settings('t1'))

        await resetTable('t1')

        expect(reset).toHaveBeenCalledWith('t1')
        expect(tables.value.map((t) => t.tableId)).toEqual(['t2'])
        expect(list).toHaveBeenCalledTimes(1)
        expect(resettingTableId.value).toBeNull()
        expect(useToast().toasts.value[0].message).toBe('Table réinitialisée aux valeurs globales.')
    })

    it('keeps the table listed when the reset is refused', async () => {
        const { tables, resetTable } = useTableSettingsOverrides()
        await flush()
        reset.mockRejectedValue(new Error('boom'))

        await resetTable('t1')

        expect(tables.value).toHaveLength(2)
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de réinitialiser la table : boom'
        })
    })
})
