import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flush } from '../test/fixtures'
import type { Version } from '../client/types'

vi.mock('../client/ops', () => ({ getVersion: vi.fn() }))

// Module-level state, like useSettings.test.ts: a fresh import per test is
// what puts the shared load under test.
async function load() {
    vi.resetModules()
    const client = await import('../client/ops')
    const { useVersion } = await import('./useVersion')
    return { client: vi.mocked(client), useVersion }
}

const version: Version = { version: 'v1.2.3', commit: 'abc1234', buildDate: '2026-08-22T00:00:00Z' }

describe('useVersion', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('loads on first use and holds nothing beforehand', async () => {
        const { client, useVersion } = await load()
        client.getVersion.mockReturnValue(new Promise(() => {}))

        const { current, loading } = useVersion()

        expect(current.value).toBeNull()
        expect(loading.value).toBe(true)
    })

    it('shares one load across every caller', async () => {
        const { client, useVersion } = await load()
        client.getVersion.mockResolvedValue(version)

        const first = useVersion()
        useVersion()
        await flush()

        expect(client.getVersion).toHaveBeenCalledTimes(1)
        expect(first.current.value).toEqual(version)
    })

    it('states a failed load in French', async () => {
        const { client, useVersion } = await load()
        client.getVersion.mockRejectedValue(new Error('boom'))

        const { error, current } = useVersion()
        await flush()

        expect(error.value).toBe('Impossible de charger la version du serveur : boom')
        expect(current.value).toBeNull()
    })
})
