import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flush } from '../test/fixtures'
import type { Settings, SettingsField } from '../client/types'
import { ApiError } from '../client/http'

vi.mock('../client/settings', () => ({ getSettings: vi.fn(), updateSettings: vi.fn() }))

function field(value: number): SettingsField {
    return { unit: 'lignes', value, minimum: 1 }
}

function settings(rowLimit: number, listLimit: number): Settings {
    return {
        defaultRowLimit: field(rowLimit),
        defaultListLimit: field(listLimit),
        defaultFeaturesLimit: field(5000),
        maxSimplifyTolerance: field(100),
        maxPrecision: field(1),
        tileCacheMaxAgeSec: field(60),
        coordinateDecimals: field(7),
        cacheTTLSec: field(300),
        updatedAt: '2026-01-01T00:00:00Z'
    }
}

// The composable holds the settings row at module level, so each test starts
// from a fresh import — the shared load is precisely what is under test.
async function load() {
    vi.resetModules()
    const client = await import('../client/settings')
    const { useSettings } = await import('./useSettings')
    const { useToast } = await import('./useToast')
    return { client: vi.mocked(client), useSettings, toasts: useToast().toasts }
}

describe('useSettings', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('serves the server’s own seed values until /settings answers', async () => {
        const { client, useSettings } = await load()
        client.getSettings.mockReturnValue(new Promise(() => {}))

        const { rowLimit, listLimit, loadedOnce } = useSettings()

        expect(rowLimit.value).toBe(100)
        expect(listLimit.value).toBe(50)
        expect(loadedOnce.value).toBe(false)
    })

    it('shares one load across every caller', async () => {
        const { client, useSettings } = await load()
        client.getSettings.mockResolvedValue(settings(200, 500))

        const first = useSettings()
        useSettings()
        await flush()

        expect(client.getSettings).toHaveBeenCalledTimes(1)
        expect(first.rowLimit.value).toBe(200)
        expect(first.listLimit.value).toBe(500)
    })

    it('makes a lowered cap visible everywhere at once', async () => {
        const { client, useSettings } = await load()
        client.getSettings.mockResolvedValue(settings(200, 500))
        const reader = useSettings()
        const writer = useSettings()
        await flush()
        client.updateSettings.mockResolvedValue(settings(50, 500))

        await writer.save({ defaultRowLimit: 50 })

        expect(reader.rowLimit.value).toBe(50)
    })

    it('states a failed load, retryable', async () => {
        const { client, useSettings } = await load()
        client.getSettings.mockRejectedValueOnce(new Error('boom'))
        const { error, loadedOnce, retry } = useSettings()
        await flush()

        expect(error.value).toBe('Impossible de charger les paramètres : boom')
        expect(loadedOnce.value).toBe(false)

        client.getSettings.mockResolvedValue(settings(100, 50))
        await retry()

        expect(loadedOnce.value).toBe(true)
        expect(error.value).toBe('')
    })

    it('saves only the fields the form changed, and confirms with a toast', async () => {
        const { client, useSettings, toasts } = await load()
        client.getSettings.mockResolvedValue(settings(100, 50))
        const { save, saving } = useSettings()
        await flush()
        client.updateSettings.mockResolvedValue(settings(200, 50))

        await expect(save({ defaultRowLimit: 200 })).resolves.toBe(true)

        expect(client.updateSettings).toHaveBeenCalledWith({ defaultRowLimit: 200 })
        expect(saving.value).toBe(false)
        expect(toasts.value.at(-1)?.message).toBe('Paramètres enregistrés.')
    })

    // The status framing errorMessage prepends ("Requête invalide : ...")
    // lands on the first segment, so that field is not attached — see
    // spec/front/todo.md TODO-F13.
    it('breaks a refusal down per field, on top of the toast', async () => {
        const { client, useSettings, toasts } = await load()
        client.getSettings.mockResolvedValue(settings(100, 50))
        const { save, fieldErrors } = useSettings()
        await flush()
        // The breakdown is read off the server's own message, which an
        // ApiError carries alone — a plain Error is prefixed by the fallback.
        client.updateSettings.mockRejectedValue(
            new ApiError({
                error: 'invalid_settings',
                message: 'defaultRowLimit must be >= 50; defaultListLimit must be <= 5000',
                code: 400,
                requestId: 'r1'
            })
        )

        await expect(save({ defaultRowLimit: 1 })).resolves.toBe(false)

        expect(fieldErrors.value.defaultListLimit).toBe('defaultListLimit must be <= 5000')
        expect(fieldErrors.value.defaultRowLimit).toBeUndefined()
        expect(toasts.value.at(-1)?.kind).toBe('error')
    })

    it('clears the previous field errors before a new save', async () => {
        const { client, useSettings } = await load()
        client.getSettings.mockResolvedValue(settings(100, 50))
        const { save, fieldErrors } = useSettings()
        await flush()
        client.updateSettings.mockRejectedValueOnce(
            new ApiError({
                error: 'invalid_settings',
                message: 'x; defaultRowLimit must be >= 50',
                code: 400,
                requestId: 'r1'
            })
        )
        await save({ defaultRowLimit: 1 })

        client.updateSettings.mockResolvedValueOnce(settings(200, 50))
        await save({ defaultRowLimit: 200 })

        expect(fieldErrors.value).toEqual({})
    })
})
