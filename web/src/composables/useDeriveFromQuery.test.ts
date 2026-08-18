import { describe, expect, it, vi } from 'vitest'
import { useDeriveFromQuery } from './useDeriveFromQuery'
import { deriveTable } from '../client/tables'
import type { Metadata } from '../client/types'

vi.mock('../client/tables', () => ({ deriveTable: vi.fn() }))

const derive = vi.mocked(deriveTable)

const created: Metadata = {
    tableId: 't9',
    physicalName: 't9',
    tableName: 'grandes_communes',
    isGeo: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    rowCount: 3,
    srid: 0
}

describe('useDeriveFromQuery', () => {
    it('derives from the bare query, scoped to no table', async () => {
        derive.mockResolvedValue(created)
        const { submit } = useDeriveFromQuery()

        await expect(submit('SELECT 1', 'grandes_communes')).resolves.toEqual(created)
        expect(derive).toHaveBeenCalledWith('SELECT 1', 'grandes_communes')
    })

    it('raises the loading flag for the attempt', async () => {
        derive.mockResolvedValue(created)
        const { isLoading, submit } = useDeriveFromQuery()

        const pending = submit('SELECT 1', 'x')
        expect(isLoading.value).toBe(true)

        await pending
        expect(isLoading.value).toBe(false)
    })

    it('states a refused query in French, answering null', async () => {
        derive.mockRejectedValue(new Error('syntaxe invalide'))
        const { error, submit } = useDeriveFromQuery()

        await expect(submit('SELEC 1', 'x')).resolves.toBeNull()
        expect(error.value).toBe('Impossible de dériver la table : syntaxe invalide')
    })
})
