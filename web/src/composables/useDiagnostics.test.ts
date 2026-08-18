import { describe, expect, it, vi } from 'vitest'
import { useDiagnostics } from './useDiagnostics'
import { withSetup, flush } from '../test/fixtures'
import { getDiagnostics } from '../client/ops'
import { ApiError } from '../client/http'
import type { Diagnostics } from '../client/types'

vi.mock('../client/ops', () => ({ getDiagnostics: vi.fn() }))

const read = vi.mocked(getDiagnostics)

const diagnostics: Diagnostics = {
    databaseSizeBytes: 1024,
    memoryUsageBytes: 2048,
    temporaryFilesBytes: 0,
    extensions: ['spatial'],
    maxSizeBytes: 0
}

describe('useDiagnostics', () => {
    it('loads on mount and holds the four states apart', async () => {
        read.mockResolvedValue(diagnostics)

        const { result } = withSetup(() => useDiagnostics())
        expect(result.loading.value).toBe(true)
        expect(result.loadedOnce.value).toBe(false)

        await flush()

        expect(result.data.value).toEqual(diagnostics)
        expect(result.loading.value).toBe(false)
        expect(result.loadedOnce.value).toBe(true)
        expect(result.error.value).toBe('')
    })

    it('states a failure in French and stays un-loaded', async () => {
        read.mockRejectedValue(new Error('boom'))

        const { result } = withSetup(() => useDiagnostics())
        await flush()

        expect(result.error.value).toBe('Impossible de charger les diagnostics du serveur : boom')
        expect(result.loadedOnce.value).toBe(false)
        expect(result.loading.value).toBe(false)
    })

    it('prefers the server’s own message', async () => {
        read.mockRejectedValue(
            new ApiError({
                error: 'forbidden',
                message: 'réservé aux administrateurs',
                code: 403,
                requestId: 'r1'
            })
        )

        const { result } = withSetup(() => useDiagnostics())
        await flush()

        expect(result.error.value).toContain('réservé aux administrateurs')
    })

    it('clears the error and reloads on retry', async () => {
        read.mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(diagnostics)
        const { result } = withSetup(() => useDiagnostics())
        await flush()

        await result.retry()

        expect(result.error.value).toBe('')
        expect(result.data.value).toEqual(diagnostics)
        expect(read).toHaveBeenCalledTimes(2)
    })
})
